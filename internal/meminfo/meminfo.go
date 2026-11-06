// Package meminfo reads the memory available to this process, respecting a
// cgroup v2 limit when present.
package meminfo

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func AvailableMB() (int, error) {
	if max, err := readInt("/sys/fs/cgroup/memory.max"); err == nil {
		if cur, err := readInt("/sys/fs/cgroup/memory.current"); err == nil && max > 0 {
			if max <= cur {
				return 0, nil
			}
			return int((max - cur) >> 20), nil
		}
	}
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) >= 2 && fields[0] == "MemAvailable:" {
			kb, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				return 0, err
			}
			return int(kb >> 10), nil
		}
	}
	return 0, fmt.Errorf("MemAvailable no encontrado")
}

func Check(requiredMB int) error {
	if requiredMB <= 0 {
		return nil
	}
	avail, err := AvailableMB()
	if err != nil {
		return nil // sin dato, no bloquees la carga
	}
	if avail < requiredMB {
		return fmt.Errorf("RAM insuficiente: el modelo pide %d MB, hay %d MB", requiredMB, avail)
	}
	return nil
}

func readInt(path string) (int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	s := strings.TrimSpace(string(data))
	if s == "max" {
		return 0, fmt.Errorf("sin límite")
	}
	return strconv.ParseInt(s, 10, 64)
}
