// 分层落点：server/game/logic（handler 只做校验与转发；校验失败**必须回校正**）
// 核心演示：移动校验 + 校正推送（协议：请求带 seq；校正带 ack_seq）
// 看什么：① 接受时**不回包**（省带宽，客户端本地预测已对）② 拒绝时**一定回校正**（否则误差累积成"被拽回"）
//         ③ 校正走**可靠**通道；④ 关键字：maxStepPerMsg（单步上限）、WalkableAt（可走性）
package logic

// def/msg.go：
//	type MoveRequest struct {
//		X, Y, Z float64 `json:"x"/"y"/"z"`   // Y 必须透传（AOI 是三维球体）
//		Seq     uint32  `json:"seq"`         // 客户端输入序号
//	}
// def/push.go：
//	const PushMoveCorrect = 3002002
//	type MoveCorrectNotify struct { AckSeq uint32 `json:"ack_seq"`; X, Y, Z float64 `json:"x"/"y"/"z"` }

func (l *gameLogic) onMove(c event.Ctx) error {
	var req def.MoveRequest
	if err := c.BindMsg(&req); err != nil {
		l.logOnce("move_bind", logger.Warnf, "mmo: Move 解析失败 player=%s err=%v", c.PlayerID(), err)
		return nil
	}
	objID, ok := sceneObjID(c.Account())
	if !ok { l.logOnce("move_pid", logger.Warnf, "mmo: Move 玩家 id 非数值 player=%q", c.PlayerID()); return nil }
	scene := l.sceneOf(objID)
	if scene == nil { l.logOnce("move_noscene_"+c.PlayerID(), logger.Warnf, "mmo: 不在任何场景 player=%s", c.PlayerID()); return nil }
	cur, ok := scene.Position(objID)
	if !ok { l.logOnce("move_nopos_"+c.PlayerID(), logger.Warnf, "mmo: 查不到位置 obj=%d", objID); return nil }

	next := mmo.Vec3{X: req.X, Y: req.Y, Z: req.Z}
	switch {
	case dist3(cur, next) > maxStepPerMsg:
		l.logOnce("move_jump_"+c.PlayerID(), logger.Warnf, "mmo: 拒绝瞬移 obj=%d 距离=%.2f>%.1f", objID, dist3(cur, next), maxStepPerMsg)
		l.pushMoveCorrect(c.PlayerID(), cur, req.Seq) // ★ 拒绝必须回校正
		return nil
	case l.mapData != nil && !l.mapData.WalkableAt(next.X, next.Z):
		l.logOnce("move_blocked_"+c.PlayerID(), logger.Warnf, "mmo: 拒绝走入不可走格 obj=%d to=(%.1f,%.1f)", objID, next.X, next.Z)
		l.pushMoveCorrect(c.PlayerID(), cur, req.Seq) // ★ 同上
		return nil
	}

	scene.Move(objID, next)
	l.pushMove(scene, objID, next) // 广播给视野内（含自己）
	return nil
}

// 校正下发：权威位置 + 已处理到的输入序号（ack）；客户端据此「丢弃已确认 + 回放未确认」。
func (l *gameLogic) pushMoveCorrect(playerID string, pos mmo.Vec3, ackSeq uint32) {
	if playerID == "" {
		return
	}
	if err := l.g.PushToPlayer(playerID, def.PushMoveCorrect,
		def.MoveCorrectNotify{AckSeq: ackSeq, X: pos.X, Y: pos.Y, Z: pos.Z}); err != nil { // 可靠通道
		l.logOnce("push_correct_fail", logger.Warnf, "mmo: 校正下发失败 player=%s err=%v（同类不再打印）", playerID, err)
	}
}
