package financial

const (
	minLimit     = 1
	maxLimit     = 500
	defaultLimit = 50
)

func normalizePage(limit, offset int) (int, int, error) {
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < minLimit || limit > maxLimit || offset < 0 {
		return 0, 0, ErrInvalid
	}
	return limit, offset, nil
}

// paginate runs after each product has computed its full result and totals.
func (list *FlatList[T]) paginate(items []T) {
	list.Total = len(items)
	if list.Offset < len(items) {
		list.Items = items[list.Offset:min(list.Offset+list.Limit, len(items))]
	}
}
