//go:build !darwin

package tun

func defaultStack(options StackOptions) (Stack, error) {
	return NewSystem(options)
}
