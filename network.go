package main

import (
	"net"
	"strings"
)

// MARK: Network

func IsLocal(ip net.IP) bool {
	if ip.IsInterfaceLocalMulticast() || ip.IsLinkLocalUnicast() || ip.IsLoopback() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	ip4 := ip.To4()

	// see https://en.wikipedia.org/wiki/Reserved_IP_addresses
	if ip4 != nil {
		if ip4[0] == 10 {  // 10.0.0.0/8
			return true
		} else if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 { // 172.16.0.0/12
			return true
		} else if ip4[0] == 192 && ip4[1] == 168 { // 192.168.0.0/16
			return true
		} else {
			return false
		}
	}
	// ip6
	return false
}

// func localInterfce() (net.Interface) {
// 	// get all interfaces
// 	interfaces, err := net.Interfaces()
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	for _, interface := range interfaces {
//
// 	}
// }

func localIP4() (*net.Interface, net.IP, error) {

	// get all interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, nil, err
	}
	for _, iface := range interfaces { // "interface" is a keyword
		// fmt.Printf("interface: %v\n", iface)
		// fmt.Println("--Unicast")
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}

		var ipString string
		for _, address := range addresses {
			// fmt.Printf("%v %v\n", address.Network(), address.String())
			switch address.Network() {
			case "ip+net":
				ipString = strings.Split(address.String(), "/")[0]
			case "ip":
				ipString = address.String()
			default:
				continue
			}

			ip, err := net.ResolveIPAddr("ip", ipString)
			if err != nil {
				continue
			}
			if IsLocal(ip.IP) {
				return &iface, ip.IP, nil
			}
		}

	}

	return nil, nil, nil

}
