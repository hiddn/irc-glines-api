package ircglineapi

import (
	"net"
	"strings"
)

// Returns ip/32 if ipv4 address provided without cidr
// Returns ip/128 if ipv6 address provided without cidr
// Returns ip unchanged if char '/' is present in string ip
func AddCidrToIP(ip string) string {
	if strings.Contains(ip, "/") {
		return ip
	}
	if tmpIP := net.ParseIP(ip); tmpIP != nil {
		if version := tmpIP.To4(); version != nil {
			ip += "/32"
		} else {
			ip += "/128"
		}
	}
	return ip
}

func StripCidrFromIP(ip string) string {
	s := strings.Split(ip, "/")
	if len(s) > 1 {
		return s[0]
	}
	return ip
}

func RemoveLastChar(w string) string {
	return w[:len(w)-1]
}

func GetFileNameFromFullPath(f string) string {
	s := strings.Split(f, "/")
	return s[len(f)-1]
}
