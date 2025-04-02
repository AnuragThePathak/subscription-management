package endpoint

func toResponseSlice[T InternalModel[R], R any](models []T) []R {
	responses := make([]R, len(models))
	for i, model := range models {
		responses[i] = model.ToResponse()
	}
	return responses
}