package bashscan

import "testing"

func BenchmarkScan_SimpleCommand(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Scan("ls -la /tmp")
	}
}

func BenchmarkScan_Pipeline(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Scan("cat file | grep foo && echo done >> log")
	}
}
