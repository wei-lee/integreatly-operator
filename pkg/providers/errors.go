package providers

type alreadyExistsErr struct {
	resource string
}

func (ae *alreadyExistsErr)Error()string  {
	return "the requested resource " + ae.resource + " already exists in the cloud provider"
}

func newAlreadyExistsError(resource string)*alreadyExistsErr  {
	return &alreadyExistsErr{
		resource:resource,
	}
}

func IsAlreadyExistsErr(err error) bool  {
	_, ok := err.(*alreadyExistsErr)
	return ok
}

type doesntExistErr struct {
	resource string
}

func newDoesntExistErr(resource string)*doesntExistErr  {
	return &doesntExistErr{resource:resource}
}
func (de *doesntExistErr)Error()string  {
	return "the requested resource " + de.resource + " does not exist"
}

func IsNotExistsErr(err error) bool  {
	_, ok := err.(*doesntExistErr)
	return ok
}

