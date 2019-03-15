package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"syscall"
)

const (
	// FHS
	ETC_DIR   = "/etc"
	SYSFS_DIR = "/sys"

	// linux: arch/x86/include/asm/unistd_64.h
	SYS_SETNS = 308

    // iproute2: include/namespace.h
	NETNS_RUN_DIR = "/var/run/netns"
	NETNS_ETC_DIR = "/etc/netns"
)

// flag: net ns name
var netNsFlag string

func init() {
	flag.StringVar(&netNsFlag, "net", "", "Network namespace name")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] [command]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
}

func runCommand(argv0 string, argv []string) error {
	cmd := exec.Command(argv0, argv...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func newMountNamespace() error {
	if err := syscall.Unshare(syscall.CLONE_NEWNS); err != nil {
		return fmt.Errorf("unshare failed: %v", err)
	}
	if err := syscall.Mount("none", "/", "", syscall.MS_REC|syscall.MS_SLAVE, ""); err != nil {
		return fmt.Errorf("mount / in namespace failed: %v", err)
	}
	return nil
}

// SetNetworkNs use setns(2) to move current proccess to specific namespace.
// It is necessary to create additional mount namespace at least for custom sysfs
func SetNetworkNs(nsName string) error {
	fdNs, err := os.Open(path.Join(NETNS_RUN_DIR, nsName))
	if err != nil {
		return fmt.Errorf("unable to find %s namespace: %v", nsName, err)
	}
	if _, _, err := syscall.RawSyscall(SYS_SETNS, fdNs.Fd(), syscall.CLONE_NEWNET, 0); err != 0 {
		fdNs.Close()
		return fmt.Errorf("got error from setns() syscall: %s", err.Error())
	}
	fdNs.Close()
	// detach from parent mount namespace
	if err := newMountNamespace(); err != nil {
		return err
	}
	// mount sysfs with related information about network devices
	if err := syscall.Unmount(SYSFS_DIR, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("umount of %s in namespace failed: %v", SYSFS_DIR, err)
	}
	if err := syscall.Mount(nsName, SYSFS_DIR, "sysfs", 0, ""); err != nil {
		return fmt.Errorf("mount of %s in namespace failed: %v", SYSFS_DIR, err)
	}
	// mount namespace configs into /etc if any exists
	nsEtcDir := path.Join(NETNS_ETC_DIR, nsName)
	dir, err := os.Open(nsEtcDir)
	// ignore if unable to read netns configs
	if err != nil {
		return nil
	} else if stat, err := dir.Stat(); err != nil || !stat.IsDir() {
		dir.Close()
		return nil
	}
	// read directory with namespace configs and make bind mount to /etc
	entries, err := dir.Readdir(-1)
	dir.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Read files in %s failed: %v", nsEtcDir, err)
		return nil
	}
	for _, file := range entries {
		srcMount := path.Join(nsEtcDir, file.Name())
		dstMount := path.Join(ETC_DIR, file.Name())
		if err := syscall.Mount(srcMount, dstMount, "none", syscall.MS_BIND, ""); err != nil {
			fmt.Fprintf(os.Stderr, "Bind mount %s -> %s failed: %g", srcMount, dstMount, err)
		}
	}
	return nil
}

func main() {
	args := flag.Args()
	if len(netNsFlag) < 1 || len(args) < 1 {
		flag.Usage()
		os.Exit(1)
	}
	if len(netNsFlag) > 0 {
		if err := SetNetworkNs(netNsFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Unable to set network namespace: %v\n", err)
			os.Exit(1)
		}
	}
	if err := runCommand(args[0], args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Get error while running command: %v\n", err)
		os.Exit(1)
	}
}
