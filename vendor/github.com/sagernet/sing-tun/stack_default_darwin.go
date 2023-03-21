package tun

func defaultStack(options StackOptions) (Stack, error) {
	if options.UnderPlatform {
		// Apple Network Extension conflicts with system stack.
		return NewGVisor(options)
	} else {
		return NewSystem(options)
	}
}
