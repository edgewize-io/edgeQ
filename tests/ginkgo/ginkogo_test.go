package ginkgo_test

import (
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Ginkogo2", func() {
	BeforeEach(func() {
		GinkgoWriter.Println("BeforeEach")
	})

	JustBeforeEach(func() {
		GinkgoWriter.Println("JustBeforeEach")
	})

	JustAfterEach(func() {
		GinkgoWriter.Println("JustAfterEach")
	})

	AfterEach(func() {
		GinkgoWriter.Println("AfterEach")
	})

	Context("RR", func() {
		BeforeEach(func() {
			GinkgoWriter.Println("+-01-BeforeEach")
		})

		JustBeforeEach(func() {
			GinkgoWriter.Println("+-01-JustBeforeEach")
		})

		JustAfterEach(func() {
			GinkgoWriter.Println("+-01-JustAfterEach")
		})

		AfterEach(func() {
			GinkgoWriter.Println("+-01-AfterEach")
		})
		It("It1", func() {
			GinkgoWriter.Println("+-01-It")
		})
	})

	Context("RR", func() {
		BeforeEach(func() {
			GinkgoWriter.Println("+-02-BeforeEach")
		})

		JustBeforeEach(func() {
			GinkgoWriter.Println("+-02-JustBeforeEach")
		})

		JustAfterEach(func() {
			GinkgoWriter.Println("+-02-JustAfterEach")
		})

		AfterEach(func() {
			GinkgoWriter.Println("+-02-AfterEach")
		})
		It("It2", func() {
			GinkgoWriter.Println("+-02-It")
		})
	})
})
