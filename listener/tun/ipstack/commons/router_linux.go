package commons

import (
	"fmt"
	"github.com/Dreamacro/clash/common/cmd"
	"github.com/Dreamacro/clash/listener/tun/device"
	"github.com/Dreamacro/clash/log"
	"net/netip"
	"runtime"
	"strconv"
)

func GetAutoDetectInterface() (string, error) {
	if runtime.GOOS == "android" {
		return cmd.ExecCmd("sh -c ip route | awk 'NR==1{print $3}' | xargs echo -n")
	}
	return cmd.ExecCmd("bash -c ip route show | grep 'default via' | awk -F ' ' 'NR==1{print $5}' | xargs echo -n")
}

func ConfigInterfaceAddress(dev device.Device, addr netip.Prefix, forceMTU int, autoRoute, autoDetectInterface bool) error {
	var (
		interfaceName = dev.Name()
		ip            = addr.Masked().Addr().Next()
	)

	_, err := cmd.ExecCmd(fmt.Sprintf("ip addr add %s dev %s", ip.String(), interfaceName))
	if err != nil {
		return err
	}

	_, err = cmd.ExecCmd(fmt.Sprintf("ip link set %s up", interfaceName))
	if err != nil {
		return err
	}

	if autoRoute {
		err = configInterfaceRouting(interfaceName, addr, autoDetectInterface)
	}
	return err
}

func configInterfaceRouting(interfaceName string, addr netip.Prefix, autoDetectInterface bool) error {
	linkIP := addr.Masked().Addr().Next()
	if runtime.GOOS == "android" {
		const tableId = 1981801
		for _, route := range defaultRoutes {
			if err := execRouterCmd("add", route, interfaceName, linkIP.String(), strconv.Itoa(tableId)); err != nil {
				return err
			}
		}
		execAddRuleCmd(fmt.Sprintf("from 0.0.0.0 iif lo uidrange 0-4294967294 lookup %d pref 9000", tableId))
		execAddRuleCmd(fmt.Sprintf("from %s iif lo uidrange 0-4294967294 lookup %d pref 9001", linkIP, tableId))
		execAddRuleCmd(fmt.Sprintf("from all iif %s lookup main suppress_prefixlength 0 pref 9002", interfaceName))
		execAddRuleCmd(fmt.Sprintf("not from all iif lo lookup %d pref 9003", tableId))

	} else {
		for _, route := range defaultRoutes {
			if err := execRouterCmd("add", route, interfaceName, linkIP.String(), "main"); err != nil {
				return err
			}
		}
	}

	if autoDetectInterface {
		go DefaultInterfaceChangeMonitor()
	}

	return nil
}

func execAddRuleCmd(rule string) {
	_, err := cmd.ExecCmd("ip rule add " + rule)
	if err != nil {
		log.Warnln("%s", err)
	}
}

func execRouterCmd(action, route, interfaceName, linkIP, table string) error {
	cmdStr := fmt.Sprintf("ip route %s %s dev %s proto kernel scope link src %s table %s", action, route, interfaceName, linkIP, table)

	_, err := cmd.ExecCmd(cmdStr)
	return err
}