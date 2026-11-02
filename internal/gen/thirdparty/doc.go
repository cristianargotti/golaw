// Package thirdparty knows the pinned generators of the enrolled
// repository's tools module (sqlc, oapi-codegen, goverter): which of them a
// policy and a checkout enable, which a change feeds, how each runs through
// go tool and how its output compares with the checkout. The kit only runs
// them as programs and never imports their code.
package thirdparty
