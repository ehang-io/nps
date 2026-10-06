package nps_mux

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"net"
	"os/exec"
	"strings"
)

type Eth struct {
	EthName string
	EthAddr string
}

type TrafficControl struct {
	Eth    *Eth
	params []string
}

func Ips() (map[string]string, error) {

	ips := make(map[string]string)

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, i := range interfaces {
		byName, err := net.InterfaceByName(i.Name)
		if err != nil {
			return nil, err
		}
		if !strings.Contains(byName.Name, "Loopback") && !strings.Contains(byName.Name, "isatap") {
			addresses, _ := byName.Addrs()
			for _, v := range addresses {
				ips[byName.Name] = v.String()
			}
		}
	}
	return ips, nil
}

// get ip and Eth information by Eth name
func GetEthByIp(ipAddr string) (eth *Eth, err error) {
	var interfaces []net.Interface
	interfaces, err = net.Interfaces()
	if err != nil {
		return
	}
	for _, i := range interfaces {
		var byName *net.Interface
		byName, err = net.InterfaceByName(i.Name)
		if err != nil {
			return
		}
		// except lo
		if !strings.Contains(byName.Name, "Loopback") && !strings.Contains(byName.Name, "isatap") && !strings.Contains(byName.Name, "lo") {
			addresses, _ := byName.Addrs()
			for _, v := range addresses {
				ipMask := strings.Split(v.String(), "/")
				if len(ipMask) == 2 {
					if ipAddr == "" || ipMask[0] == ipAddr {
						eth = new(Eth)
						eth.EthAddr = ipMask[0]
						eth.EthName = byName.Name
						return
					}
				}
			}
		}
	}
	err = errors.New("not found interface")
	return
}

type tcFunc func()

func getArrayExhaustivity(arr []tcFunc) (result [][]tcFunc) {
	var l = int(math.Pow(float64(2), float64(len(arr))) - 1)
	var t []tcFunc
	for i := 1; i <= l; i++ {
		s := i
		t = []tcFunc{}
		for k := 0; s > 0; k++ {
			if s&1 == 1 {
				t = append(t, arr[k])
			}
			s >>= 1
		}
		result = append(result, t)
	}
	return
}

func NewTrafficControl(ipAddr string) (*TrafficControl, error) {
	Eth, err := GetEthByIp(ipAddr)
	if err != nil {
		return nil, err
	}
	t := new(TrafficControl)
	t.params = make([]string, 0)
	t.Eth = Eth
	return t, nil
}

// test the network randomly
func (tc *TrafficControl) RunNetRangeTest(f func()) error {
	funcs := tc.getTestVariable()
	groups := getArrayExhaustivity(funcs)
	for _, v := range groups {
		tc.del()
		// execute bandwidth control, not good work
		//if err := tc.bandwidth("1mbit"); err != nil {
		//	return err
		//}
		// execute random strategy
		for _, vv := range v {
			vv()
		}
		err := tc.Run()
		if err != nil {
			return err
		}
		// execute test func
		f()
		// clear strategy
		if err := tc.del(); err != nil {
			return err
		}
	}
	return nil
}

// create test variables
func (tc *TrafficControl) getTestVariable() []tcFunc {
	return []tcFunc{
		func() { tc.delay("add", "100ms", "10ms", "30%") },
		func() { tc.loss("add", "1%", "30%") },
		func() { tc.duplicate("add", "1%") },
		func() { tc.corrupt("add", "0.2%") },
	}
}

// this command sets the transmission of the network card to delayVal. At the same time,
// about waveRatio of the packets will be delayed by ± wave.
func (tc *TrafficControl) delay(opt, delayVal, wave, waveRatio string) {
	tc.params = append(tc.params, []string{"delay", delayVal, wave, waveRatio,}...)
}

// this command sets the transmission of the network card to randomly drop lossRatio of packets with a success rate of lossSuccessRatio.
func (tc *TrafficControl) loss(opt, lossRatio, lossSuccessRatio string) {
	tc.params = append(tc.params, []string{"loss", lossRatio, lossSuccessRatio,}...)
}

// this command sets the transmission of the network card to randomly generate repeatRatio duplicate packets
func (tc *TrafficControl) duplicate(opt, duplicateRatio string) {
	tc.params = append(tc.params, []string{"duplicate", duplicateRatio,}...)
}

// this command sets the transmission of the network card to randomly generate corruptRatio corrupted packets.
// the kernel version must be above 2.6.16
func (tc *TrafficControl) corrupt(opt, corruptRatio string) {
	tc.params = append(tc.params, []string{"corrupt", corruptRatio,}...)
}

func (tc *TrafficControl) Run() error {
	tc.params = append([]string{"qdisc", "add", "dev", tc.Eth.EthName, "root", "netem",}, tc.params...)
	return runCmd(exec.Command("tc", tc.params...))
}

// remove all tc setting
func (tc *TrafficControl) del() error {
	tc.clear()
	return runCmd(exec.Command("tc", "qdisc", "del", "dev", tc.Eth.EthName, "root"))
}

// clear the old settings
func (tc *TrafficControl) clear() {
	tc.params = tc.params[:0]
}

// remove all tc setting
func (tc *TrafficControl) bandwidth(bw string) error {
	runCmd(exec.Command("tc", "qdisc", "add", "dev", tc.Eth.EthName, "root", "handle", "2:", "htb", "default", "30"))
	return runCmd(exec.Command("tc", "qdisc", "add", "dev", tc.Eth.EthName, "parent", "2:", "classid", "2:30", "htb", "rate", bw))
}

func runCmd(cmd *exec.Cmd) error {
	fmt.Println("run cmd:", cmd.Args)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
		return err
	}
	return nil
}

func createNetwork(name, networok string) error {
	// docker network create --subnet=172.18.0.0/16 test
	return runCmd(exec.Command("docker", "network", "create", "--subnet="+networok, name))
}

func deleteNetwork(name string) error {
	return runCmd(exec.Command("docker", "network", "rm", name))
}

func runDocker(dockerName, networkName, ip, testFunName, nowDir string) error {
	// docker run --env GOPROXY=https://goproxy.cn  --rm --name client --net test --cap-add=NET_ADMIN --ip 172.18.0.5 -v "$PWD":/usr/src/myapp -w /usr/src/myapp golang go test -v -run TestClient ./
	return runCmd(exec.Command("docker", "run", "--env", "GOPROXY=https://goproxy.cn", "--rm", "--name", dockerName, "--net", networkName,
		"--cap-add=NET_ADMIN", "--ip", ip, "-v", nowDir+`:/usr/src/myapp`, "-w", `/usr/src/myapp`, "golang", "go", "test",
		"-v", "-run", testFunName, "./"))
}

func stopDocker(dockerName string) error {
	return runCmd(exec.Command("docker", "stop", dockerName))
}
