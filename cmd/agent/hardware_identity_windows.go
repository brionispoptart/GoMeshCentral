//go:build windows

package main

import "golang.org/x/sys/windows/registry"

func readMachineID() string {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()
	value, _, err := key.GetStringValue("MachineGuid")
	if err != nil {
		return ""
	}
	return value
}

func readSystemID() string {
	return readBIOSValue("SystemProductName", "SystemSKU", "SystemSerialNumber")
}

func readBoardID() string {
	return readBIOSValue("BaseBoardSerialNumber", "BaseBoardProduct")
}

func readBIOSValue(names ...string) string {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `HARDWARE\DESCRIPTION\System\BIOS`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()
	for _, name := range names {
		value, _, err := key.GetStringValue(name)
		if err == nil && value != "" {
			return value
		}
	}
	return ""
}
