package app

// Mount keeps the happy path and drops the error.
func Mount(resolve func() (string, error), use func(string)) {
	if dir, err := resolve(); err == nil {
		use(dir)
	}
}
