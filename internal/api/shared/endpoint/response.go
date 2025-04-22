package endpoint

// InternalModel defines a generic interface for models that support conversion to response types.
type InternalModel[T any] interface {
	ToResponse() *T
}

// ToResponse converts an internal model to its response type.
func ToResponse[T InternalModel[R], R any](model T, err error) (*R, error) {
	if err != nil {
		return nil, err
	}
	return model.ToResponse(), nil
}

// ToResponseSlice converts a slice of internal models to a slice of response types.
func ToResponseSlice[T InternalModel[R], R any](models []T, err error) ([]*R, error) {
	if err != nil {
		return nil, err
	}
	responses := make([]*R, len(models))
	for i, model := range models {
		responses[i] = model.ToResponse()
	}
	return responses, nil
}
