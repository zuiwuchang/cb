package ipv4

import "net"

func Next(ip net.IP) net.IP {
	next := ip.To4()
	if next != nil {
		a, b, c, d := next[0], next[1], next[2], next[3]
		if d < 255 {
			d++
		} else {
			d = 0
			if c < 255 {
				c++
			} else {
				c = 0
				if b < 255 {
					b++
				} else {
					b = 0
					if d < 255 {
						d++
					} else {
						return nil
					}
				}
			}
		}
		next[0], next[1], next[2], next[3] = a, b, c, d
	}
	return next
}
