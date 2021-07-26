package nject

import (
	"fmt"
)

func ExampleCluster() {
	chain := Sequence("overall",
		func() string {
			return "example string"
		},
		Cluster("first-cluster",
			func(s string) int32 {
				return int32(len(s))
			},
			func(i int32) {
				fmt.Println("auto-desired in 1st cluster")
			},
			func(i int32) int64 {
				return int64(i)
			},
		),
		Cluster("second-cluster",
			func(s string) uint32 {
				return uint32(len(s))
			},
			func(i uint32) {
				fmt.Println("auto-desired in 2nd cluster")
			},
			func(i int64, u uint32) uint64 {
				return uint64(uint32(i) + u)
			},
		),
	)
	_ = Run("does not consume uint64",
		chain,
		func(s string) {
			fmt.Println("no need for data from clusters")
		},
	)
	_ = Run("consumes uint64",
		chain,
		func(u uint64) {
			fmt.Println("got value that needed both chains -", u)
		},
	)
	// Output: no need for data from clusters
	// auto-desired in 1st cluster
	// auto-desired in 2nd cluster
	// got value that needed both chains - 28
}
