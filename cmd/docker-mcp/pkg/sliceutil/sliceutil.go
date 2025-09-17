package sliceutil

// Map returns a new slice with the results of applying the given function to
// each item in the input slice.
//
// This function comes from https://github.com/moby/moby and is subject to the
// Apache 2.0 license. See:
//
// - https://github.com/moby/moby/blob/82ca3ccaf349ce2df15d19f61f21af480999fccd/internal/sliceutil/sliceutil.go#L18-L24
// - https://github.com/moby/moby/blob/82ca3ccaf349ce2df15d19f61f21af480999fccd/LICENSE
func Map[S ~[]In, In, Out any](s S, fn func(In) Out) []Out {
	res := make([]Out, len(s))
	for i, v := range s {
		res[i] = fn(v)
	}
	return res
}

// Filter returns a slice of items which satisfies the predicate.
func Filter[S ~[]In, In any](s S, predicate func(In) bool) S {
	var res S
	for _, v := range s {
		if predicate(v) {
			res = append(res, v)
		}
	}
	return res
}
