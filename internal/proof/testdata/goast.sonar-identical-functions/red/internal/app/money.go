package app

// Cart holds prices.
type Cart struct {
	Prices []int
}

// Total adds the prices.
func (c Cart) Total() int {
	sum := 0
	for _, p := range c.Prices {
		sum += p
	}
	return sum
}

// Sum adds the prices again.
func (c Cart) Sum() int {
	sum := 0
	for _, p := range c.Prices {
		sum += p
	}
	return sum
}
