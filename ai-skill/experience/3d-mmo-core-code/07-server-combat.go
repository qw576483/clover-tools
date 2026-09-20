// 分层落点：server/game/logic（一个系统一个文件组：combat.go = 战斗状态 + 结算 + 推送 + 限频）
// 核心演示：服务端权威战斗的**四件决定性的事** —— 少了任何一件，战斗要么算错、要么链路走不到
// 看什么：① 多返回值顺序（曾因接错位置造成 99 点秒杀）② 唯一结算入口 ③ 接收者=观看者 ④ 靶子要"够得着、会还手"
package logic

// ---------- ① 属性读取：顺序即契约，接错位置编译器不报错 ----------

// combatStats 返回 (hp, maxHp, atk, def)。
// ⚠️ 曾写成 `atk, _, _, _, ok := combatStats(...)` —— atk 拿到的是 hp(100)，
//    伤害变成 100-1=99 一次秒杀。编译/vet/公式单测全绿，只有运行时日志 dmg=99 hp=0/60 才暴露。
//    调用点必须对照本注释，并用"期望数值"写死测试（打假人 19、4 下击杀）。
func (l *gameLogic) combatStats(objID uint64) (hp, maxHp, atk, def int32, ok bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	st := l.combat[objID]
	if st == nil {
		return 0, 0, 0, 0, false
	}
	return st.hp, st.maxHp, st.atk, st.def, true
}

// damage 伤害公式：max(1, atk - def/2)。保底 1 让任何攻击都有正反馈；
// def/2 用整数除法 —— 两端若要显示预览必须用同一算法。
func damage(atk, def int32) int32 {
	if d := atk - def/2; d > 1 {
		return d
	}
	return 1
}

// ---------- ② 唯一结算入口：玩家打人 / 靶子还手走同一套 ----------
//
// 抽公共函数的理由：两条路径各写一遍，迟早漂移（一条改了公式另一条没改），
// 而且"改了血量却没推属性/没判死亡"这类漏项只会出现在其中一条上。
func (l *gameLogic) applyDamage(scene mmo.Scene, attacker, target uint64, tpos mmo.Vec3) (dmg, hp, maxHp int32, killed, ok bool) {
	_, _, atk, _, okA := l.combatStats(attacker) // ← 顺序：atk 是第 3 个
	if !okA {
		return 0, 0, 0, false, false
	}

	l.mu.Lock()
	tgt := l.combat[target]
	if tgt == nil || tgt.dead {
		l.mu.Unlock()
		return 0, 0, 0, false, false
	}
	dmg = damage(atk, tgt.def)
	tgt.hp -= dmg
	if tgt.hp < 0 {
		tgt.hp = 0
	}
	hp, maxHp, isBot := tgt.hp, tgt.maxHp, tgt.isBot
	killed = hp <= 0
	if killed {
		tgt.dead = true
		if isBot {
			tgt.respawnAt = time.Now().Add(respawnDelay)
		}
	}
	l.mu.Unlock()

	// 命中表现：广播战斗事件 + 属性变化（属性推送走引擎 EPushDataSync + {"property":{...}}）
	l.pushCombat(scene, attacker, def.CombatNotify{Kind: "hit", Attacker: attacker, Target: target,
		Damage: dmg, TargetHp: hp, TargetMaxHp: maxHp, X: tpos.X, Y: tpos.Y, Z: tpos.Z})
	l.pushProperty(scene, target, map[string]any{"hp": hp})

	// 靶子记仇：被打过的机器人会还手 → 玩家**才会死** → 死亡/复活链路才可达（见 ④）
	if isBot {
		l.mu.Lock()
		if cur, ok := l.bots[target]; ok && cur.target != attacker {
			cur.target = attacker
			cur.nextHit = time.Now().Add(botAttackInterval) // 留一个"打得过就撤"的窗口
		}
		l.mu.Unlock()
	}
	if killed {
		l.pushCombat(scene, attacker, def.CombatNotify{Kind: "dead", Attacker: attacker, Target: target, X: tpos.X, Y: tpos.Y, Z: tpos.Z})
		if isBot {
			scene.Leave(target) // 玩家**不**移出场景：他要留在原地等自己发 MsgRespawn
		}
	}
	return dmg, hp, maxHp, killed, true
}

// ---------- ③ 推送接收者 = 观看者（不是 scene.Around 的全部对象）----------
//
// `scene.Around` 返回视野内**所有对象**（含假人），而假人没有连接。
// 把假人 id 丢给 PushToPlayer 不报错、却会被投递出去 ⇒
// 客户端**同一条事件收到 N 份**（实测：1 玩家 + 3 假人 ⇒ 一次命中客户端打 4 条日志）。
// 纪律：先算"谁在看"，只推给**真实玩家**；移动/属性/战斗三条推送共用这一个函数。
func (l *gameLogic) viewerIDs(scene mmo.Scene, center uint64) []string {
	ids := scene.Around(center, viewRadius)
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if !l.isPlayer(id) { // playerScene 由进图时的 bindScene 登记
			continue
		}
		out = append(out, strconv.FormatUint(id, 10))
	}
	return out
}

// ---------- ④ 靶子要"够得着、打得到、死得了" ----------
//
// 三个都缺一不可（少任何一个，战斗闭环都形同虚设）：
//   1. 巡逻半径：靶子不能一路走掉（走掉 = 按 B 刷在身边十几秒后就够不着）
//   2. 记仇反击：靶子不还手 ⇒ 玩家一个人打靶**永远死不了** ⇒ 死亡/复活代码永远没被执行过
//   3. 原地重生：在 botState.home 重生，而不是"按 id 轮转到另一个出生点"（刚打死的靶子换个地方消失）
func (l *gameLogic) botTick() {
	now := time.Now()
	for i := range snapshot {
		b := snapshot[i]
		if l.botRespawn(b, now) {
			continue // 死亡冷却中 / 刚重生
		}
		if b.target != 0 {
			l.botTryHit(scene, b, now) // 反击（到冷却/超距/目标已死都有明确处理与日志）
		}
		next := mmo.Vec3{X: b.pos.X + b.dirX*b.step, Y: b.pos.Y, Z: b.pos.Z}
		if b.patrol > 0 && math.Abs(next.X-b.home.X) > b.patrol { // 巡逻边界折返
			b.dirX = -b.dirX
		}
		// …撞墙折返 + scene.Move + pushMove
	}
}
