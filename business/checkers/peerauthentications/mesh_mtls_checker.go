package peerauthentications

import (
	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/models"
)

// Note that MeshMtlsChecker will work with MeshPolicy resources
type MeshMtlsChecker struct {
	MeshPolicy    kubernetes.IstioObject
	MTLSDetails   kubernetes.MTLSDetails
	IsServiceMesh bool
}

func (t MeshMtlsChecker) Check() ([]*models.IstioCheck, bool) {
	validations := make([]*models.IstioCheck, 0)

	// if MeshPolicy doesn't have mtls in strict mode, stop validation with any check.
	if strictMode := kubernetes.PeerAuthnHasStrictMTLS(t.MeshPolicy); !strictMode {
		return validations, true
	}

	// otherwise, check among Destination Rules for a rule enabling mTLS mesh-wide.
	for _, dr := range t.MTLSDetails.DestinationRules {
		if enabled, _ := kubernetes.DestinationRuleHasMeshWideMTLSEnabled(dr); enabled {
			return validations, true
		}
	}

	check := models.Build("peerauthentication.mtls.destinationrulemissing", "spec/mtls")
	validations = append(validations, &check)

	return validations, false
}
