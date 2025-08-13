package gonet

import (
	"context"
	"go.uber.org/zap"
	"net"
	"slgGame/pkg/log"
	"slgGame/pkg/pool"
)

type ReadHandler func(data []byte)

type tcpConn struct {
	write       chan []byte
	conn        net.Conn
	read        chan []byte
	readHandler ReadHandler
}

func (t *tcpConn) Write() {
	pool.Go(func(ctx context.Context) (interface{}, error) {
		for {
			select {
			case <-ctx.Done():
				return nil, nil
			case data := <-t.write:
				t.writeData(data)
			}
		}

		return nil, nil
	})
}

func (t *tcpConn) writeData(data []byte) {

}

func (t *tcpConn) Read() {
	pool.Go(func(ctx context.Context) (interface{}, error) {
		for {
			select {
			case <-ctx.Done():
				return nil, nil
			case data := <-t.read:
				t.readHandler(data)
			}
		}

		return nil, nil
	})
}

func (t *tcpConn) ReadHandler() {}

func (t *tcpConn) Run() {
	if t.conn != nil {
		return
	}
	t.Write()
	t.Read()
	log.Info("tcp init run", zap.String("addr", t.conn.RemoteAddr().String()))
}
