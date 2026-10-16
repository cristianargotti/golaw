package application

type row struct {
	Message string
}

// Sync stores the error text in the row.
func Sync(load func() error) row {
	var r row
	if err := load(); err != nil {
		r.Message = err.Error()
	}
	return r
}
