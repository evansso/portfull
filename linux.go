package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Listener struct {
	Port    int
	Address string
	PID     int
	Command string
}

func linuxListeners() ([]Listener, error) {
	inodes := make(map[string]socketAddress)
	for _, table := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		file, err := os.Open(table)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read %s: %w", table, err)
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 10 || fields[3] != "0A" { // TCP_LISTEN
				continue
			}
			port, err := strconv.ParseInt(fields[1][strings.LastIndex(fields[1], ":")+1:], 16, 32)
			if err != nil {
				continue
			}
			inode := fields[9]
			addr, err := decodeProcAddress(fields[1])
			if err == nil {
				inodes[inode] = socketAddress{port: int(port), address: addr}
			}
		}
		scanErr := scanner.Err()
		_ = file.Close()
		if scanErr != nil {
			return nil, fmt.Errorf("read %s: %w", table, scanErr)
		}
	}
	if len(inodes) == 0 {
		return []Listener{}, nil
	}

	listeners := make([]Listener, 0)
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("scan processes: %w", err)
	}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		fds, err := os.ReadDir(filepath.Join("/proc", entry.Name(), "fd"))
		if err != nil {
			continue
		}
		owned := make(map[string]bool)
		for _, fd := range fds {
			target, err := os.Readlink(filepath.Join("/proc", entry.Name(), "fd", fd.Name()))
			if err != nil || !strings.HasPrefix(target, "socket:[") {
				continue
			}
			inode := strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")
			if _, ok := inodes[inode]; ok {
				owned[inode] = true
			}
		}
		if len(owned) == 0 {
			continue
		}
		commandBytes, _ := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		command := strings.TrimSpace(strings.ReplaceAll(string(commandBytes), "\x00", " "))
		if command == "" {
			commandBytes, _ = os.ReadFile(filepath.Join("/proc", entry.Name(), "comm"))
			command = strings.TrimSpace(string(commandBytes))
		}
		for inode := range owned {
			socket := inodes[inode]
			listeners = append(listeners, Listener{Port: socket.port, Address: socket.address, PID: pid, Command: command})
		}
	}
	sort.Slice(listeners, func(i, j int) bool {
		if listeners[i].Port != listeners[j].Port {
			return listeners[i].Port < listeners[j].Port
		}
		if listeners[i].PID != listeners[j].PID {
			return listeners[i].PID < listeners[j].PID
		}
		return listeners[i].Address < listeners[j].Address
	})
	return listeners, nil
}

type socketAddress struct {
	port    int
	address string
}

func decodeProcAddress(value string) (string, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid socket address %q", value)
	}
	port, err := strconv.ParseInt(parts[1], 16, 32)
	if err != nil {
		return "", err
	}
	if len(parts[0]) == 8 {
		packed, err := strconv.ParseUint(parts[0], 16, 32)
		if err != nil {
			return "", err
		}
		var raw [4]byte
		binary.LittleEndian.PutUint32(raw[:], uint32(packed))
		return net.JoinHostPort(net.IP(raw[:]).String(), strconv.Itoa(int(port))), nil
	}
	if len(parts[0]) == 32 {
		var raw [16]byte
		for i := 0; i < 4; i++ {
			word, err := strconv.ParseUint(parts[0][i*8:(i+1)*8], 16, 32)
			if err != nil {
				return "", err
			}
			binary.LittleEndian.PutUint32(raw[i*4:(i+1)*4], uint32(word))
		}
		return net.JoinHostPort(net.IP(raw[:]).String(), strconv.Itoa(int(port))), nil
	}
	return "", fmt.Errorf("unexpected address length in %q", value)
}
