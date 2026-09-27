package ui

import "golang.org/x/sys/windows"

func init() {
	enableVirtualTerminal = func(fd uintptr) error {
		handle := windows.Handle(fd)
		var mode uint32
		if err := windows.GetConsoleMode(handle, &mode); err != nil {
			return err
		}
		if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
			return nil
		}
		return windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	}
}
