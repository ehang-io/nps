module ehang.io/nps

go 1.13

require (
	ehang.io/nps-mux v0.0.0-20200109142326-674a17784f79
	fyne.io/fyne v1.2.0
	github.com/astaxie/beego v1.12.0
	github.com/c4milo/unpackit v0.0.0-20170704181138-4ed373e9ef1c
	github.com/ccding/go-stun v0.0.0-20180726100737-be486d185f3d
	github.com/golang/snappy v0.0.0-20180518054509-2e65f85255db
	github.com/kardianos/service v1.0.0
	github.com/klauspost/cpuid v1.2.2 // indirect
	github.com/klauspost/reedsolomon v1.9.3 // indirect
	github.com/panjf2000/ants/v2 v2.2.2
	github.com/pkg/errors v0.8.1
	github.com/shirou/gopsutil v2.19.11+incompatible
	github.com/templexxx/xor v0.0.0-20191217153810-f85b25db303b // indirect
	github.com/tjfoc/gmsm v1.2.0 // indirect
	github.com/xtaci/kcp-go v5.4.20+incompatible
	golang.org/x/crypto v0.0.0-20200108215511-5d647ca15757 // indirect
	golang.org/x/net v0.0.0-20191209160850-c0dbc17a3553
	golang.org/x/sys v0.0.0-20200107162124-548cf772de50 // indirect
)

replace github.com/astaxie/beego => github.com/exfly/beego v1.12.0-export-init
