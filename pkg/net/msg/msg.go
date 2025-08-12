package msg

// 包头固定4+4字节
type baseMsgHead struct {
	idx    int32  // 包索引号  4
	packet int32  // 包长度    4
	msgNum int32  // 协议号    4
	error  uint16 // 错误码   2
}

type baseMsg struct {
	head baseMsgHead // 包头部
	body []byte      // 包体
}
