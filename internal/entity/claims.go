package entity

type KeycloakClaims struct {
	Sub               string                    `json:"sub"`
	Email             string                    `json:"email"`
	EmailVerified     bool                      `json:"email_verified"`
	PreferredUsername string                    `json:"preferred_username"`
	Name              string                    `json:"name"`
	GivenName         string                    `json:"given_name"`
	FamilyName        string                    `json:"family_name"`
	RealmAccess       RealmAccess               `json:"realm_access"`
	ResourceAccess    map[string]ResourceAccess `json:"resource_access"`
}

type RealmAccess struct {
	Roles []string `json:"roles"`
}

type ResourceAccess struct {
	Roles []string `json:"roles"`
}

func (c *KeycloakClaims) HasRealmRole(role string) bool {
	for _, r := range c.RealmAccess.Roles {
		if r == role {
			return true
		}
	}
	return false
}

func (c *KeycloakClaims) HasClientRole(clientID, role string) bool {
	access, ok := c.ResourceAccess[clientID]
	if !ok {
		return false
	}
	for _, r := range access.Roles {
		if r == role {
			return true
		}
	}
	return false
}
