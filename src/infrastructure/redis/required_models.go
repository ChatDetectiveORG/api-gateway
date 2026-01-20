package redis

// RequiredModels is a starter template for Redis keys.
//
// Customize it freely:
// - key names / prefixes
// - seed strategy (or remove seeding and only verify key types)
// - expirations
//
// Tip: running InitRedis(RequiredModels) on every boot is fine; operations are idempotent.
var RequiredModels = []Model{
	EnsureStringKey{
		Key: "api-gateway:router:version",
		Seed: "1",
	},
}
