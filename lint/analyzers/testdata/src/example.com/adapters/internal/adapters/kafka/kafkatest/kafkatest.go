// A test package may expose the SDK.
package kafkatest

import "github.com/twmb/franz-go/pkg/kgo"

func Client() *kgo.Client { return nil }
