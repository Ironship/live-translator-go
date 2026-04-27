//go:build windows

package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

const manifestFileName = "live-translator-go.exe.manifest"

var (
	kernel32              = syscall.NewLazyDLL("kernel32.dll")
	createActCtxW         = kernel32.NewProc("CreateActCtxW")
	activateActCtx        = kernel32.NewProc("ActivateActCtx")
	deactivateActCtx      = kernel32.NewProc("DeactivateActCtx")
	releaseActCtx         = kernel32.NewProc("ReleaseActCtx")
	invalidActCtxHandle   = ^uintptr(0)
	errManifestNotPresent = errors.New("manifest file not present")
)

type actCtx struct {
	cbSize                uint32
	flags                 uint32
	source                *uint16
	processorArchitecture uint16
	languageID            uint16
	assemblyDirectory     *uint16
	resourceName          *uint16
	applicationName       *uint16
	module                syscall.Handle
}

type manifestActivation struct {
	handle uintptr
	cookie uintptr
}

func activateManifestContext() (*manifestActivation, error) {
	path, err := resolveManifestPath()
	if err != nil {
		if errors.Is(err, errManifestNotPresent) {
			return nil, nil
		}
		return nil, err
	}

	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	ctx := actCtx{
		cbSize: uint32(unsafe.Sizeof(actCtx{})),
		source: pathPtr,
	}

	handle, _, callErr := createActCtxW.Call(uintptr(unsafe.Pointer(&ctx)))
	if handle == invalidActCtxHandle {
		return nil, fmt.Errorf("create activation context from %s: %w", path, callErr)
	}

	var cookie uintptr
	ok, _, callErr := activateActCtx.Call(handle, uintptr(unsafe.Pointer(&cookie)))
	if ok == 0 {
		releaseActCtx.Call(handle)
		return nil, fmt.Errorf("activate manifest context from %s: %w", path, callErr)
	}

	return &manifestActivation{handle: handle, cookie: cookie}, nil
}

func (a *manifestActivation) Close() {
	if a == nil || a.handle == 0 {
		return
	}
	if a.cookie != 0 {
		deactivateActCtx.Call(0, a.cookie)
		a.cookie = 0
	}
	releaseActCtx.Call(a.handle)
	a.handle = 0
}

func resolveManifestPath() (string, error) {
	if cwd, err := os.Getwd(); err == nil {
		path := filepath.Join(cwd, manifestFileName)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}

	if executablePath, err := os.Executable(); err == nil {
		path := filepath.Join(filepath.Dir(executablePath), manifestFileName)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}

	return "", errManifestNotPresent
}
