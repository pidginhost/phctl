package cmdutil

// FetchAll iterates through all pages of a paginated API endpoint.
// The fetch function receives a page number (starting at 1) and returns
// the results for that page, whether there are more pages, and any error.
func FetchAll[T any](fetch func(page int32) ([]T, bool, error)) ([]T, error) {
	var all []T
	for page := int32(1); ; page++ {
		results, hasNext, err := fetch(page)
		if err != nil {
			return nil, err
		}
		all = append(all, results...)
		if !hasNext {
			break
		}
	}
	return all, nil
}
