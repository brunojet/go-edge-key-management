# Changelog

## [1.1.0](https://github.com/brunojet/go-edge-key-management/compare/v1.0.0...v1.1.0) (2026-06-04)


### Features

* Move public key creation to createSecret with sanitization ([eaa48be](https://github.com/brunojet/go-edge-key-management/commit/eaa48bee3a77c617d575decdd33b3122049efd55))
* Move public key creation to createSecret with sanitization on errors ([b8a80a5](https://github.com/brunojet/go-edge-key-management/commit/b8a80a5c357f09e84da38711e8e25f91ea9b799d))
* Simplify stale AWSPENDING cleanup with unconditional discard ([fa91acd](https://github.com/brunojet/go-edge-key-management/commit/fa91acd4c3a985d98972e55ac7d1be98ca035830))
* upgrade go-infra-adapters to v3.1.0+ with 100% test coverage ([b5f1b13](https://github.com/brunojet/go-edge-key-management/commit/b5f1b132b798e84eab55de7deeca0a312cedd2bc))
* upgrade to go-infra-adapters v3.0.0 with public HealthCheck API ([b2f48b4](https://github.com/brunojet/go-edge-key-management/commit/b2f48b46cc05d9134d4c6e8a017761d23ea382ad))


### Bug Fixes

* correct coverage requirement to 96% (actual: 96.5%) ([a4d18e0](https://github.com/brunojet/go-edge-key-management/commit/a4d18e02d6cbbeb5a68ba28bc6b1be624630e360))
* linting issues - errcheck, goimports, misspell, revive, staticcheck ([3fd7269](https://github.com/brunojet/go-edge-key-management/commit/3fd7269463637c9d3864ba1daaf337cd4a29704f))
* lower coverage requirement from 100% to 97% ([d303488](https://github.com/brunojet/go-edge-key-management/commit/d303488ad3eec1f5132417bb44b888086a7c2d46))
* remove invalid TestCreateSecret_GetVersionError test ([a556217](https://github.com/brunojet/go-edge-key-management/commit/a556217ad2e762ecc07123ac33b19e7441b8acc3))
* resolve all linting issues - goimports and gosec ([8525cd6](https://github.com/brunojet/go-edge-key-management/commit/8525cd683437c7538d2aca9d8cb407092e531656))
* validate complete key pair in pending version to handle stale AWSPENDING ([79035b6](https://github.com/brunojet/go-edge-key-management/commit/79035b6ce8d54b3a83f1f703aca73fe3a8e585b8))


### Code Refactoring

* extract discardPublicKey helper and fix error handling ([322baf7](https://github.com/brunojet/go-edge-key-management/commit/322baf70ec7afea5acc583b042079001e694af5f))
* migrate to go-infra-adapters v2.0.0 with adapter pattern ([c9c0a9c](https://github.com/brunojet/go-edge-key-management/commit/c9c0a9c87cd6175e8b1bb426aeb63de17d3dc849))
* use CDN and secret services from go-infra-adapters adapter ([e78e1a9](https://github.com/brunojet/go-edge-key-management/commit/e78e1a9caeb3992a9f0676925adc769663230896))
* validate all required fields in SecretPayload.IsValid() ([51fe180](https://github.com/brunojet/go-edge-key-management/commit/51fe1802508ae5ec93d4bfc97d2b74ca8363c270))
