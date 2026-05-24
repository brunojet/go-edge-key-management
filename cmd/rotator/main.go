//go:build !debug
// +build !debug

package main

import "github.com/aws/aws-lambda-go/lambda"

func main() {
	lambda.Start(RotationHandler)
}
