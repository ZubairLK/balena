package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	_ "github.com/docker/docker/daemon/graphdriver/aufs"
	_ "github.com/docker/docker/daemon/graphdriver/overlay2"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/mount"
	"golang.org/x/sys/unix"
)

const (
	LAYER_ROOT = "/balena"
	PIVOT_PATH = "/mnt/sysroot/active"
)

func mountContainer(sysroot string) string {
	rawGraphDriver, err := ioutil.ReadFile(filepath.Join(sysroot, "/current/boot/storage-driver"))
	if err != nil {
		log.Fatal("could not get storage driver:", err)
	}
	graphDriver := strings.TrimSpace(string(rawGraphDriver))

	current, err := os.Readlink(filepath.Join(sysroot, "/current"))
	if err != nil {
		log.Fatal("could not get container ID:", err)
	}
	containerID := filepath.Base(current)

	layer_root := filepath.Join(sysroot, LAYER_ROOT)
	ls, err := layer.NewStoreFromOptions(layer.StoreOptions{
		StorePath:                 layer_root,
		MetadataStorePathTemplate: filepath.Join(layer_root, "image", "%s", "layerdb"),
		IDMappings:                &idtools.IDMappings{},
		GraphDriver:               graphDriver,
		OS:                        "linux",
	})
	if err != nil {
		log.Fatal("error loading layer store:", err)
	}

	rwlayer, err := ls.GetRWLayer(containerID)
	if err != nil {
		log.Fatal("error getting container layer:", err)
	}

	newRoot, err := rwlayer.Mount("")
	if err != nil {
		log.Fatal("error mounting container fs:", err)
	}
	newRootPath := newRoot.Path()

	if err := unix.Mount("", newRootPath, "", unix.MS_REMOUNT, ""); err != nil {
		log.Fatal("error remounting container as read/write:", err)
	}

	return newRootPath
}

func prepareForPivot(mounts []*mount.Info, newRootPath string) {
	if err := os.MkdirAll(filepath.Join(newRootPath, PIVOT_PATH), os.ModePerm); err != nil {
		log.Fatal("creating /mnt/sysroot failed:", err)
	}

	unix.Mount("", newRootPath, "", unix.MS_REMOUNT|unix.MS_RDONLY, "")
	unix.Unmount("/dev/shm", unix.MNT_DETACH)

	for _, mount := range mounts {
		if mount.Mountpoint == "/" {
			continue
		}
		if err := unix.Mount(mount.Mountpoint, filepath.Join(newRootPath, mount.Mountpoint), "", unix.MS_MOVE, ""); err != nil {
			log.Println("could not move mountpoint:", mount.Mountpoint, err)
		}
	}
}

func main() {
	sysrootPtr := flag.String("sysroot", "", "root of partition e.g. /mnt/sysroot/inactive. Mount destination is returned in stdout")
	flag.Parse()
	var mounts []*mount.Info
	var err error

	// If a custom sysroot is not passed, we prepare before mounting aufs
	if *sysrootPtr == "" {
		// Any mounts done by initrd will be transfered in the new root
		mounts, err = mount.GetMounts()
		if err != nil {
			log.Fatal("could not get mounts:", err)
		}

		if err := unix.Mount("", "/", "", unix.MS_REMOUNT, ""); err != nil {
			log.Fatal("error remounting root as read/write:", err)
		}

		if err := os.MkdirAll("/dev/shm", os.ModePerm); err != nil {
			log.Fatal("creating /dev/shm failed:", err)
		}

		if err := unix.Mount("shm", "/dev/shm", "tmpfs", 0, ""); err != nil {
			log.Fatal("error mounting /dev/shm:", err)
		}
	}

	newRootPath := mountContainer(*sysrootPtr)

	// If a custom sysroot is passed, we print newRootPath to stdout and don't pivot.
	if *sysrootPtr != "" {
		fmt.Print(newRootPath)
	} else {
		prepareForPivot(mounts, newRootPath)

		if err := syscall.PivotRoot(newRootPath, filepath.Join(newRootPath, PIVOT_PATH)); err != nil {
			log.Fatal("error while pivoting root:", err)
		}

		if err := unix.Chdir("/"); err != nil {
			log.Fatal(err)
		}

		if err := syscall.Exec("/sbin/init", os.Args, os.Environ()); err != nil {
			log.Fatal("error executing init:", err)
		}
	}
}
