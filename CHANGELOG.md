# Changelog

## [0.9.2](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.9.1...tfclassify-v0.9.2) (2026-09-29)


### Bug Fixes

* **ci:** bump Go to 1.25.14, pin govulncheck, sync generated protobuf headers ([6bd7ec4](https://github.com/jokarl/tfclassify/commit/6bd7ec4c8697ebbbfa9c6e957a6d394c252dae02))
* **deps:** migrate to gobwas/glob v1 Pattern type ([61e4830](https://github.com/jokarl/tfclassify/commit/61e4830b3cea43c28772910935c2a59e470bae03))
* refresh Azure role data and action registry ([#177](https://github.com/jokarl/tfclassify/issues/177)) ([e43ec0d](https://github.com/jokarl/tfclassify/commit/e43ec0d5083c9d95013f370ceeddad88e6b9f269))


### Dependencies

* **deps:** bump github.com/gobwas/glob from 0.2.3 to 1.0.0 ([#229](https://github.com/jokarl/tfclassify/issues/229)) ([b525a37](https://github.com/jokarl/tfclassify/commit/b525a3798e44aaddf990220ea77037f49335939a))
* **deps:** bump github.com/hashicorp/go-plugin from 1.7.0 to 1.8.0 ([#180](https://github.com/jokarl/tfclassify/issues/180)) ([7644dcd](https://github.com/jokarl/tfclassify/commit/7644dcdf73ef6808dbced9d9ee398d323cdf25ba))
* **deps:** bump github.com/hashicorp/terraform-json from 0.27.2 to 0.27.3 ([#204](https://github.com/jokarl/tfclassify/issues/204)) ([926f129](https://github.com/jokarl/tfclassify/commit/926f1291cb6817b3e20710117dcbc293dd5a7245))
* **deps:** bump github.com/hashicorp/terraform-json from 0.27.3 to 0.28.0 ([#210](https://github.com/jokarl/tfclassify/issues/210)) ([4aea3a6](https://github.com/jokarl/tfclassify/commit/4aea3a633c124b5847bb4bf48a768cb4ab78cb41))
* **deps:** bump github.com/zclconf/go-cty from 1.18.1 to 1.19.0 ([#208](https://github.com/jokarl/tfclassify/issues/208)) ([b96c30a](https://github.com/jokarl/tfclassify/commit/b96c30a8d3fe886d3edffd880d64837319334c16))
* **deps:** bump google.golang.org/grpc from 1.80.0 to 1.81.0 ([#185](https://github.com/jokarl/tfclassify/issues/185)) ([1316c2c](https://github.com/jokarl/tfclassify/commit/1316c2c6b3e22cc74d042aa9e6f94b89a7a3ff6f))
* **deps:** bump google.golang.org/grpc from 1.80.0 to 1.81.0 in /sdk ([#184](https://github.com/jokarl/tfclassify/issues/184)) ([90a886e](https://github.com/jokarl/tfclassify/commit/90a886e9d86a12a899eacb4344cae22f9ebe5698))
* **deps:** bump google.golang.org/grpc from 1.81.0 to 1.81.1 ([#189](https://github.com/jokarl/tfclassify/issues/189)) ([b37ff99](https://github.com/jokarl/tfclassify/commit/b37ff99ccffb1881d59008c9734ff09cd01c77c2))
* **deps:** bump google.golang.org/grpc from 1.81.0 to 1.81.1 in /sdk ([#188](https://github.com/jokarl/tfclassify/issues/188)) ([7d00dc7](https://github.com/jokarl/tfclassify/commit/7d00dc7b3e741ea19447e18724de72a1dd2d2ad6))
* **deps:** bump google.golang.org/grpc from 1.81.1 to 1.82.0 ([#203](https://github.com/jokarl/tfclassify/issues/203)) ([ab36fdf](https://github.com/jokarl/tfclassify/commit/ab36fdfebe6e15e37ce965e9cd887e0cdb2c1b5a))
* **deps:** bump google.golang.org/grpc from 1.81.1 to 1.82.0 in /sdk ([#202](https://github.com/jokarl/tfclassify/issues/202)) ([19731ab](https://github.com/jokarl/tfclassify/commit/19731abba42e2802fa7b932fa7d995fe23c51d79))
* **deps:** bump google.golang.org/grpc from 1.82.0 to 1.82.1 ([#209](https://github.com/jokarl/tfclassify/issues/209)) ([b253094](https://github.com/jokarl/tfclassify/commit/b253094455f8855a9d3d9439f19781b9470a665f))
* **deps:** bump google.golang.org/grpc from 1.82.0 to 1.82.1 in /sdk ([#207](https://github.com/jokarl/tfclassify/issues/207)) ([53d99e2](https://github.com/jokarl/tfclassify/commit/53d99e2522252318d867baca4b7df1a768cb6652))
* **deps:** bump google.golang.org/grpc from 1.82.1 to 1.83.0 ([#217](https://github.com/jokarl/tfclassify/issues/217)) ([715a149](https://github.com/jokarl/tfclassify/commit/715a149a2c866e40f93aaa5eda1a999fbd952674))
* **deps:** bump google.golang.org/grpc from 1.82.1 to 1.83.0 in /sdk ([#216](https://github.com/jokarl/tfclassify/issues/216)) ([e058b8e](https://github.com/jokarl/tfclassify/commit/e058b8eb546d5fc78d53adc04111623c77ab1298))
* **deps:** bump google.golang.org/grpc from 1.83.0 to 1.83.1 ([#223](https://github.com/jokarl/tfclassify/issues/223)) ([f2b513b](https://github.com/jokarl/tfclassify/commit/f2b513b74c761324fc9fd0f2b21040a04900e53f))
* **deps:** bump google.golang.org/grpc from 1.83.1 to 1.83.2 ([#226](https://github.com/jokarl/tfclassify/issues/226)) ([77cf1cf](https://github.com/jokarl/tfclassify/commit/77cf1cf1e97051d918b90a0124c20373642c659e))
* **deps:** bump google.golang.org/protobuf from 1.36.11 to 1.36.12 ([#219](https://github.com/jokarl/tfclassify/issues/219)) ([69e4444](https://github.com/jokarl/tfclassify/commit/69e44440edd404b999a5048255b1e53658c0db29))

## [0.9.1](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.9.0...tfclassify-v0.9.1) (2026-04-28)


### Bug Fixes

* refresh Azure role data and action registry ([#171](https://github.com/jokarl/tfclassify/issues/171)) ([2351b19](https://github.com/jokarl/tfclassify/commit/2351b191fde65c93b3fea47ffb3de7a6e030399c))
* warn on module glob overlap and skip fixture-only verify scenarios ([#174](https://github.com/jokarl/tfclassify/issues/174)) ([94390da](https://github.com/jokarl/tfclassify/commit/94390dab62bdff590f1cac9308fb9814f0063635))


### Dependencies

* **deps:** bump github.com/zclconf/go-cty from 1.18.0 to 1.18.1 ([#169](https://github.com/jokarl/tfclassify/issues/169)) ([1306afe](https://github.com/jokarl/tfclassify/commit/1306afe6db00ddddd0aedcb2fbc5b2700cc43e65))

## [0.9.0](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.8.1...tfclassify-v0.9.0) (2026-04-21)


### Features

* scoped ignore attributes ([#167](https://github.com/jokarl/tfclassify/issues/167)) ([2ce7d08](https://github.com/jokarl/tfclassify/commit/2ce7d08093aed6f31079d9db59c1c070f8547d2d))
* skip no-op rule evaluation ([#168](https://github.com/jokarl/tfclassify/issues/168)) ([3a8f040](https://github.com/jokarl/tfclassify/commit/3a8f0404c200eeec0a7bba75248115fa2cb274f3))


### Bug Fixes

* refresh Azure role data and action registry ([#165](https://github.com/jokarl/tfclassify/issues/165)) ([ebb5cc5](https://github.com/jokarl/tfclassify/commit/ebb5cc59f3d57132f34b194ee4f95b9f7c02bd85))


### Dependencies

* **deps:** bump github.com/hashicorp/go-version from 1.8.0 to 1.9.0 ([#163](https://github.com/jokarl/tfclassify/issues/163)) ([d368a0e](https://github.com/jokarl/tfclassify/commit/d368a0e5c928ed948e82364d3a0222bc1b7d2627))
* **deps:** bump google.golang.org/grpc from 1.79.3 to 1.80.0 ([#164](https://github.com/jokarl/tfclassify/issues/164)) ([8ce328b](https://github.com/jokarl/tfclassify/commit/8ce328bc1e516863056e87cd2c91e0005aa25491))
* **deps:** bump google.golang.org/grpc from 1.79.3 to 1.80.0 in /sdk ([#162](https://github.com/jokarl/tfclassify/issues/162)) ([0014dd9](https://github.com/jokarl/tfclassify/commit/0014dd9f369bdf7e891abe994025d812b5396e4e))

## [0.8.1](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.8.0...tfclassify-v0.8.1) (2026-04-13)


### Bug Fixes

* refresh Azure role data and action registry ([#159](https://github.com/jokarl/tfclassify/issues/159)) ([0e03195](https://github.com/jokarl/tfclassify/commit/0e03195f2d8cd7e9d3c493cea83bad539a5d2538))

## [0.8.0](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.7.5...tfclassify-v0.8.0) (2026-04-08)


### Features

* add scaffold command to generate starter config from terraform state ([c82a3df](https://github.com/jokarl/tfclassify/commit/c82a3dfa4c049d7d833102094c103e77037bd4cb))


### Bug Fixes

* **ci:** use compact output flag for jq matrix discovery ([bea5824](https://github.com/jokarl/tfclassify/commit/bea5824c4308aaa81885661ead1f42ba339e9d32))
* refresh Azure role data and action registry ([#155](https://github.com/jokarl/tfclassify/issues/155)) ([cd3fb84](https://github.com/jokarl/tfclassify/commit/cd3fb8455f18c4f79a8bfebcaf452cc08568efc3))

## [0.7.5](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.7.4...tfclassify-v0.7.5) (2026-03-30)


### Bug Fixes

* refresh Azure role data and action registry ([#152](https://github.com/jokarl/tfclassify/issues/152)) ([baad7f8](https://github.com/jokarl/tfclassify/commit/baad7f81b24b511262fdc7733c3fc8bd89ecfd35))

## [0.7.4](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.7.3...tfclassify-v0.7.4) (2026-03-27)


### Bug Fixes

* hide no-op resources in text output, show only active changes ([351bc74](https://github.com/jokarl/tfclassify/commit/351bc7462b44f8df6b8862fd79b35c675bb13cec))

## [0.7.3](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.7.2...tfclassify-v0.7.3) (2026-03-27)


### Bug Fixes

* show downgraded resources in verbose no-changes output ([acba5e8](https://github.com/jokarl/tfclassify/commit/acba5e8ef78f758cf0173051e39d28a716d4a6c4))

## [0.7.2](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.7.1...tfclassify-v0.7.2) (2026-03-26)


### Bug Fixes

* show "No resource changes" instead of classification description ([9ba052b](https://github.com/jokarl/tfclassify/commit/9ba052bf2b7a5639583d05f7f10a2a1f74ee0579))

## [0.7.1](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.7.0...tfclassify-v0.7.1) (2026-03-26)


### Bug Fixes

* treat all-no-op plans as no_changes after cosmetic filtering ([#144](https://github.com/jokarl/tfclassify/issues/144)) ([ef539c2](https://github.com/jokarl/tfclassify/commit/ef539c2412e7bb8b5678f9b9efe8771bf3d9efb8))

## [0.7.0](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.6.0...tfclassify-v0.7.0) (2026-03-25)


### Features

* add ignore_attributes to exclude cosmetic changes from classifi… ([#138](https://github.com/jokarl/tfclassify/issues/138)) ([85af62c](https://github.com/jokarl/tfclassify/commit/85af62c97a80f880bac749c87bdb809d5b804ad2))


### Bug Fixes

* refresh Azure role data and action registry ([#70](https://github.com/jokarl/tfclassify/issues/70)) ([d0b9390](https://github.com/jokarl/tfclassify/commit/d0b9390b465997e654de70c27b8029948464ac07))
* refresh Azure role data and action registry ([#98](https://github.com/jokarl/tfclassify/issues/98)) ([c3af999](https://github.com/jokarl/tfclassify/commit/c3af999d0b5694f8bf8113584869866995bcf306))
* use unique scope in combined-role-aggregation e2e to prevent parallel CI collisions ([53e9c09](https://github.com/jokarl/tfclassify/commit/53e9c0949e0642f651676df7e216a8940dc4ca7e))


### Dependencies

* **deps:** bump github.com/zclconf/go-cty from 1.17.0 to 1.18.0 ([#67](https://github.com/jokarl/tfclassify/issues/67)) ([38bde0a](https://github.com/jokarl/tfclassify/commit/38bde0a0e84ee815a98ebfa6fb9f5e4860b50279))
* **deps:** bump golang.org/x/sync from 0.19.0 to 0.20.0 ([#97](https://github.com/jokarl/tfclassify/issues/97)) ([6fabb0c](https://github.com/jokarl/tfclassify/commit/6fabb0cea89e394374d8348246b2c9c0ef5bc9dd))
* **deps:** bump google.golang.org/grpc from 1.79.1 to 1.79.3 ([#137](https://github.com/jokarl/tfclassify/issues/137)) ([aa24c7c](https://github.com/jokarl/tfclassify/commit/aa24c7c12599da13ade0d839c456d3ea0abf0d38))
* **deps:** bump google.golang.org/grpc from 1.79.1 to 1.79.3 in /sdk ([#136](https://github.com/jokarl/tfclassify/issues/136)) ([e90c7fc](https://github.com/jokarl/tfclassify/commit/e90c7fc91df295476f0e708cc2a1e9a475bd19ce))

## [0.6.0](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.5.2...tfclassify-v0.6.0) (2026-02-26)


### Features

* add combined exposure analysis ([#64](https://github.com/jokarl/tfclassify/issues/64)) ([569c32b](https://github.com/jokarl/tfclassify/commit/569c32b663facfb78c9c70df8ce710ec58b47657))
* add module-scoped rules, drift classification, and topology analysis ([#65](https://github.com/jokarl/tfclassify/issues/65)) ([87b33bd](https://github.com/jokarl/tfclassify/commit/87b33bd3a78750abdec93d3a1758b897f6a88dee))


### Bug Fixes

* remove shallow analyzers, fix code defects, harden testing ([#62](https://github.com/jokarl/tfclassify/issues/62)) ([74b15c4](https://github.com/jokarl/tfclassify/commit/74b15c4e0dfa45288c7c6b8c062f8c441918b364))

## [0.5.2](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.5.1...tfclassify-v0.5.2) (2026-02-23)


### Bug Fixes

* refresh Azure role data and action registry ([#51](https://github.com/jokarl/tfclassify/issues/51)) ([a132079](https://github.com/jokarl/tfclassify/commit/a132079321681dcb00d7fed24d7fe6e32c10f952))


### Dependencies

* **gomod:** bump github.com/hashicorp/go-version from 1.7.0 to 1.8.0 ([#46](https://github.com/jokarl/tfclassify/issues/46)) ([cf733d8](https://github.com/jokarl/tfclassify/commit/cf733d8befa100cc505c6b1cd10c87c77f0f7049))
* **gomod:** bump github.com/zclconf/go-cty from 1.16.4 to 1.17.0 ([#47](https://github.com/jokarl/tfclassify/issues/47)) ([477cf04](https://github.com/jokarl/tfclassify/commit/477cf046f86391a9d9319de81ecbaf5106fdcc2b))

## [0.5.1](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.5.0...tfclassify-v0.5.1) (2026-02-22)


### Bug Fixes

* address v0.5.0 critique findings ([#42](https://github.com/jokarl/tfclassify/issues/42)) ([74abb72](https://github.com/jokarl/tfclassify/commit/74abb72dc035d0dda0c3fc93b1970b154330f974))

## [0.5.0](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.4.0...tfclassify-v0.5.0) (2026-02-22)


### Features

* SARIF 2.1.0 output format (CR-0033) ([#39](https://github.com/jokarl/tfclassify/issues/39)) ([f23854d](https://github.com/jokarl/tfclassify/commit/f23854dae8be20bf5eae562b8c14aba29f13c9cc))

## [0.4.0](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.3.0...tfclassify-v0.4.0) (2026-02-20)


### Features

* blast radius analyzer, not_actions, evidence artifacts (CR-0029/0030/0032) ([#37](https://github.com/jokarl/tfclassify/issues/37)) ([1db2990](https://github.com/jokarl/tfclassify/commit/1db29906fc7a2bec2af2d658c69cd80bfd2d735f))

## [0.3.0](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.2.1...tfclassify-v0.3.0) (2026-02-18)


### Features

* add validate and explain subcommands (CR-0025, CR-0026) ([#35](https://github.com/jokarl/tfclassify/issues/35)) ([012ea82](https://github.com/jokarl/tfclassify/commit/012ea820109756a1cfb78fcfbca0cfbd22fc90fa))

## [0.2.1](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.2.0...tfclassify-v0.2.1) (2026-02-18)


### Performance Improvements

* optimize hot paths and reduce allocations across codebase ([#31](https://github.com/jokarl/tfclassify/issues/31)) ([8ffe569](https://github.com/jokarl/tfclassify/commit/8ffe56999bde1523d2c623946c01a7a902181842))

## [0.2.0](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.1.0...tfclassify-v0.2.0) (2026-02-17)


### Features

* classification-scoped plugin config and --detailed-exitcode flag ([#14](https://github.com/jokarl/tfclassify/issues/14)) ([128646b](https://github.com/jokarl/tfclassify/commit/128646b8d33bb64cf585bc3d9460bee98a96d924))
* pattern-based control-plane and data-plane detection (CR-0027, CR-0028) ([#22](https://github.com/jokarl/tfclassify/issues/22)) ([3b4ef50](https://github.com/jokarl/tfclassify/commit/3b4ef50f37ad4aade4dadb03abb2e655a3de52fe))

## [0.1.0](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.0.1...tfclassify-v0.1.0) (2026-02-14)


### Features

* implement tfclassify MVP with plugin architecture, Azure deep inspection, and CI/CD ([#2](https://github.com/jokarl/tfclassify/issues/2)) ([8d15cd6](https://github.com/jokarl/tfclassify/commit/8d15cd65748b2465b96ff1af93fce90598dcf84b))


### Bug Fixes

* bump Go to 1.26.0 to resolve 16 stdlib vulnerabilities ([d02e40e](https://github.com/jokarl/tfclassify/commit/d02e40e690ce0cd000054108852568251809c377))
* honor TERRAFORM_PATH override when path does not exist ([6d9d09a](https://github.com/jokarl/tfclassify/commit/6d9d09a57ed2f9cddcc6c76d5b5f3c9b3edd7aae))
