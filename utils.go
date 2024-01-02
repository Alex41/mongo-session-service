package session_service

func searchNotNil(i ...error) error {
	for _, k := range i {
		if k != nil {
			return k
		}
	}
	return nil
}
