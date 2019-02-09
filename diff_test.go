package main_test

import (
	"bytes"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/rightscale/right_st"
)

var _ = Describe("Diff", func() {
	DescribeTable("without time",
		func(bytesA, bytesB []byte, diff bool, output string) {
			t := time.Time{}
			d, o, err := Diff("a", "b", t, t, bytes.NewReader(bytesA), bytes.NewReader(bytesB))
			Expect(err).NotTo(HaveOccurred())
			Expect(d).To(Equal(diff))
			Expect(o).To(Equal(output))
		},
		Entry("identical text", []byte("a"), []byte("a"), false, ""),
		Entry("identical binary", []byte{0x1f, 0x8b, 0x08}, []byte{0x1f, 0x8b, 0x08}, false, ""),
		Entry("differing text", []byte(`a
b
c
d
e
f
g
h
i
j
k
l
m
n
o
p
q
r
s
t
u
v
w
x
y
z
`), []byte(`a
2
3
d
e
f
g
h
i
j
11
l
13
14
o
p
q
r
s
t
u
v
23
24
25
z
`), true, `--- a
+++ b
@@ -1,6 +1,6 @@
 a
-b
-c
+2
+3
 d
 e
 f
@@ -8,10 +8,10 @@
 h
 i
 j
-k
+11
 l
-m
-n
+13
+14
 o
 p
 q
@@ -20,8 +20,8 @@
 t
 u
 v
-w
-x
-y
+23
+24
+25
 z
 
`),
		Entry("differing binary", []byte{0x1f, 0x8b, 0x08}, []byte{0x50, 0x4B, 0x03, 0x04}, true, "Binary files a and b differ\n"),
		Entry("binary then text", []byte{0}, []byte("a"), true, "Binary files a and b differ\n"),
		Entry("text then binary", []byte("a"), []byte{0}, true, "Binary files a and b differ\n"),
	)

	It("should output timestamps when they are given", func() {
		tA := time.Date(2018, 12, 24, 0, 0, 0, 0, time.UTC)
		tB := time.Date(2018, 12, 25, 0, 0, 0, 0, time.UTC)
		rA := bytes.NewReader([]byte(`a
b
c
d
`))
		rB := bytes.NewReader([]byte(`a
2
3
d
`))
		diff, output, err := Diff("a", "b", tA, tB, rA, rB)
		Expect(err).NotTo(HaveOccurred())
		Expect(diff).To(Equal(true))
		Expect(output).To(Equal(`--- a	2018-12-24T00:00:00Z
+++ b	2018-12-25T00:00:00Z
@@ -1,5 +1,5 @@
 a
-b
-c
+2
+3
 d
 
`))
	})
})
