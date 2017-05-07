
// +build linux darwin dragonfly freebsd netbsd openbsd solaris

package main

import "syscall"

func checkDirWritable (dir string) error {
	return syscall.Access(dir,2)
}
