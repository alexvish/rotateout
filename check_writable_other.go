
// +build !linux,!darwin,!dragonfly,!freebsd,!netbsd,!openbsd,!solaris

package main

func checkDirWritable (dir string) error {
	return nil;
}
