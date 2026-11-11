package domain

import "github.com/michaelolof/enumx"

type Asset string // want `Asset needs ParseAsset\(s string\) \(Asset, error\), the one way in from untrusted text \(DOM-6\)` `Asset needs a Validate\(\) error method \(DOM-6\)` `Asset needs a Oneof\(\) string method, built from a sorted helper \(DOM-6\)`

var fallback = Asset("USD")

var Assets, setAsset, _ = enumx.New[Asset, any](&fallback) // want `an enum's fallback is nil, so the zero value is invalid: enumx.New\[Asset, …\]\(nil\) \(DOM-6\)`

var AssetUSD = setAsset("USD", nil)

type Priority int // want `Priority is an enum, so it's string-kinded \(DOM-6\)` `Priority needs ParsePriority` `Priority needs a Validate` `Priority needs a Oneof`

var Priorities, setPriority, _ = enumx.New[Priority, any](nil)

var PriorityHigh = setPriority(1, nil)
