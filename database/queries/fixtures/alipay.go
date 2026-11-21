package fixtures

import _ "embed"

//go:embed alipay/create_membership_reader.sql
var CreateMembershipReader string

//go:embed alipay/create_membership_session.sql
var CreateMembershipSession string

//go:embed alipay/count_paid_terms.sql
var CountPaidTerms string
