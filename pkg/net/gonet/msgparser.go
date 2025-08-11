package gonet

/*
	如何抽象网络接口呢，应该有哪些呢：
	再平常使用中：
	1、tcp的read、write
	2、粘包问题：通过协议头部定义包体长度来解决
	3、协议路由？？需要在这儿一层吗 感觉应该有个单独的一层
*/

// 链接抽象
type conn interface {
	Write()
	Read()
	Run()
}

// 协议解析器
type msgParser interface {
	parse(data []byte) []byte
}
