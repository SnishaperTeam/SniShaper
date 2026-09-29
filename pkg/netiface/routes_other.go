//go:build !windows && !linux && !darwin

package netiface

func lookupDefaultRoutes(family int) []defaultRoute {
	return nil
}
