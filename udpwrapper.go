package udpwrapper

import (
	"log"
	"errors"
	"net"
	"sync"
	"time"
)

type (
	UDPConnWithAddr struct {
		udpListener *UDPListener
		remoteAddr net.Addr
		// there is no actual conn in udp, so we use a timeout to fake conn Close
		ttl		   time.Duration
		lastTime   time.Time
		// msg nil indicate this conn is closed
		msg        [][]byte
		connCloseCh chan struct{}
	}

	UDPListener struct {
		udpConn  *net.UDPConn
		conns    sync.Map
		acceptCh chan *UDPConnWithAddr
		closeCh  chan struct{}
		err      error
	}
)

func NewUDPlistener(addr string, port int) (*UDPListener, error) {
	listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(addr), Port: port})
	if err != nil {
		return nil, err
	}
	acceptCh := make(chan *UDPConnWithAddr)
	closeCh := make(chan struct{})
	var conns sync.Map
	l := &UDPListener{
		udpConn:  listener,
		conns:    conns,
		acceptCh: acceptCh,
		closeCh:  closeCh,
	}
	return l, nil
}

func (l *UDPListener) Listen() {
	for {
		select {
		case <-l.closeCh:
			return
		default:
			break
		}
		b := make([]byte, 1536)
		n, addr, err := l.udpConn.ReadFrom(b)
		if err != nil {
			l.err = err
			log.Println(err)
			// close(l.closeCh)
		} else {
			if v, ok := l.conns.Load(addr.String()); ok {
				log.Println("there is  add in map", addr, l.conns)
				if udpConnWA, ok := v.(*UDPConnWithAddr); ok {
					udpConnWA.msg = append(udpConnWA.msg, b[:n])
				} else {
					log.Printf("[ERROR] not *UDPConnWithAddr")
				}
			} else {
				udpConnWA := &UDPConnWithAddr{
					udpListener: l,
					remoteAddr:  addr,
					ttl:		 time.Duration(0),
					lastTime:    time.Now(),
					msg:         make([][]byte, 0),
					connCloseCh: make(chan struct{}),
				}
				udpConnWA.SetTimeout(15*time.Second)
				udpConnWA.msg = append(udpConnWA.msg, b[:n])
				l.conns.Store(addr.String(), udpConnWA)

				l.acceptCh <- udpConnWA
			}
		}
	}
}

func (l *UDPListener) Accept() (net.Conn, error) {
	if l.udpConn == nil {
		return nil, errors.New("udp listener closed")
	}
	select {
	case conn, ok := <-l.acceptCh:
		if ok {
			return conn, nil
		} else {
			return nil, errors.New("udp listener channel closed")
		}
	case <-l.closeCh:
		return nil, errors.New("udp listener closed")
	}
}

func (l *UDPListener) Close() error {
	close(l.acceptCh)
	close(l.closeCh)
	err := l.udpConn.Close()
	l.udpConn = nil
	return err
}

func (l *UDPListener) Addr() net.Addr {
	return l.udpConn.LocalAddr()
}

func (u *UDPConnWithAddr) SetTimeout(t time.Duration) error {
	u.ttl = t
	// connCloseCh
	if t > 0 {
		go func(){
			ticker := time.NewTicker(t/2)
			defer ticker.Stop()
			for{
				select{
				case<- ticker.C:
					if time.Now().Sub(u.lastTime) >  u.ttl {
						u.Close()
						return
					}
				case<- u.connCloseCh:
					return
				}
			}
		}() 
		return nil
	} else {
		return errors.New("timeout should > 0")
	}

}

func (u *UDPConnWithAddr) SetDeadline(t time.Time) error {
	return nil
}

func (u *UDPConnWithAddr) SetReadDeadline(t time.Time) error {
	return nil
}

func (u *UDPConnWithAddr) SetWriteDeadline(t time.Time) error {
	return nil
}

func (u *UDPConnWithAddr) LocalAddr() net.Addr {
	return u.udpListener.udpConn.LocalAddr()
}

func (u *UDPConnWithAddr) RemoteAddr() net.Addr {
	return u.remoteAddr
}

func (u *UDPConnWithAddr) Read(b []byte) (int, error) {
	if u.msg != nil {
		if len(u.msg) > 0 {
			n := copy(b, u.msg[0])
			u.msg = u.msg[1:]
			u.lastTime = time.Now()
			return n, nil
		} else {
			return 0, nil
		}
	} else {
		return 0, nil
	}
}

func (u *UDPConnWithAddr) Write(b []byte) (int, error) {
	if u.msg != nil {
		u.lastTime = time.Now()
		return u.udpListener.udpConn.WriteTo(b, u.remoteAddr)
	} else {
		return 0, errors.New("UDPConnWithAddr is closed")
	}
}

func (u *UDPConnWithAddr) Close() error {
	u.msg = nil
	u.udpListener.conns.Delete(u.remoteAddr.String())
	// u.udpListener = nil
	close(u.connCloseCh)
	return nil
}

// func main() {
// 	l, err :=  NewUDPlistener("0.0.0.0", 514)
// 	if err != nil {
// 		log.Println(err)
// 	}
// 	go l.Listen()
	
// 	for{
// 		conn, err :=l.Accept()	
// 		if err != nil {
// 			log.Println(err)
// 			continue
// 		} 
// 		log.Println("accept new conn", conn, err)
// 		go func(){
// 			time.Sleep(15*time.Second)
// 			l.Close()
// 		}()
// 		go func(conn net.Conn){
// 			log.Println(conn.RemoteAddr(), conn.LocalAddr())
// 			for{
// 				data := make([]byte, 1024)
// 				n,err := conn.Read(data)
// 				if n> 0{
// 					log.Println(11,n, err, string(data[:n]))
// 					if string(data[:n]) == "exit" {
// 						n, err = conn.Write([]byte("success"))
// 						conn.Close()
// 					}
// 					n, err = conn.Write([]byte(conn.RemoteAddr().String()))
// 					log.Println(22,n, err)
// 				}
// 				if err != nil {
// 					log.Println(11,n, err, string(data[:n]))
// 					return
// 				}
// 			}
// 		}(conn)
// 	}
// }
