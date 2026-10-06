# nps-mux
A net connection multiplexing implementation base on golang

# Key feature:
 - Code division
 - Conn interface implements
 - Lock free queue
 - Slide window
 - Tcp like reliable stream connection based on
 
# Usage
1. Dial or Accept a `net.Conn`
    - client:
    `c_client := net.Dial("tcp", "127.0.0.1:8024")`
    - server:
    `listener, err := net.Listen("tcp", "127.0.0.1:8024")`
    `c_server, err := listener.Accept()`
1. Make connection to mux connection
    - client:
    `mux_client := nps_mux.NewMux(c_client, "tcp", 60)`
    - server:
    `mux_server := nps_mux.NewMux(c_server, "tcp", 60)`

1. You can handle new connections both side, like this
    - client:
    `newConn, err := mux_client.Accept()`
    - server:
    `clientConn, err := mux_server.NewConn()`

`newConn` and `clientConn` are transfer data though mux connection

You can use Read Write method to transfer your own data

# More
See [mux_test.go](https://github.com/ehang-io/nps-mux/blob/master/mux_test.go)