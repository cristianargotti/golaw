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

// Max finds the highest price.
func (c Cart) Max() int {
	best := 0
	for _, p := range c.Prices {
		best = max(best, p)
	}
	return best
}
