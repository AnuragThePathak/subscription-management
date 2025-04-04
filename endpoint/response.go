package endpoint

// InternalModel is a generic interface for models that support conversion.
type InternalModel[T any] interface {
	ToResponse() *T
}

// ToResponse converts an internal model (with a conversion method) to its response type.
// It takes the result of an operation (model, error) and returns the response or propagates the error.
// Note that T must of type pointer because ToResponse method of models are defined on their pointer.
func ToResponse[T InternalModel[R], R any](model T, err error) (*R, error) {
	if err != nil {
		return nil, err
	}
	response := model.ToResponse()
	return response, nil
}

// toResponseSlice converts a slice of internal models to a slice of response types.
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
