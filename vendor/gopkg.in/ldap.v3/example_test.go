package ldap

import (
	"crypto/tls"
	"fmt"
	"log"
)

// ExampleConn_Bind demonstrates how to bind a connection to an ldap user
// allowing access to restricted attributes that user has access to
func ExampleConn_Bind() {
	l, err := Dial("tcp", fmt.Sprintf("%s:%d", "ldap.example.com", 389))
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	err = l.Bind("cn=read-only-admin,dc=example,dc=com", "password")
	if err != nil {
		log.Fatal(err)
	}
}

// ExampleConn_Search demonstrates how to use the search interface
func ExampleConn_Search() {
	l, err := Dial("tcp", fmt.Sprintf("%s:%d", "ldap.example.com", 389))
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	searchRequest := NewSearchRequest(
		"dc=example,dc=com", // The base dn to search
		ScopeWholeSubtree, NeverDerefAliases, 0, 0, false,
		"(&(objectClass=organizationalPerson))", // The filter to apply
		[]string{"dn", "cn"},                    // A list attributes to retrieve
		nil,
	)

	sr, err := l.Search(searchRequest)
	if err != nil {
		log.Fatal(err)
	}

	for _, entry := range sr.Entries {
		fmt.Printf("%s: %v\n", entry.DN, entry.GetAttributeValue("cn"))
	}
}

// ExampleStartTLS demonstrates how to start a TLS connection
func ExampleConn_StartTLS() {
	l, err := Dial("tcp", fmt.Sprintf("%s:%d", "ldap.example.com", 389))
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	// Reconnect with TLS
	err = l.StartTLS(&tls.Config{InsecureSkipVerify: true})
	if err != nil {
		log.Fatal(err)
	}

	// Operations via l are now encrypted
}

// ExampleConn_Compare demonstrates how to compare an attribute with a value
func ExampleConn_Compare() {
	l, err := Dial("tcp", fmt.Sprintf("%s:%d", "ldap.example.com", 389))
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	matched, err := l.Compare("cn=user,dc=example,dc=com", "uid", "someuserid")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(matched)
}

func ExampleConn_PasswordModify_admin() {
	l, err := Dial("tcp", fmt.Sprintf("%s:%d", "ldap.example.com", 389))
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	err = l.Bind("cn=admin,dc=example,dc=com", "password")
	if err != nil {
		log.Fatal(err)
	}

	passwordModifyRequest := NewPasswordModifyRequest("cn=user,dc=example,dc=com", "", "NewPassword")
	_, err = l.PasswordModify(passwordModifyRequest)

	if err != nil {
		log.Fatalf("Password could not be changed: %s", err.Error())
	}
}

func ExampleConn_PasswordModify_generatedPassword() {
	l, err := Dial("tcp", fmt.Sprintf("%s:%d", "ldap.example.com", 389))
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	err = l.Bind("cn=user,dc=example,dc=com", "password")
	if err != nil {
		log.Fatal(err)
	}

	passwordModifyRequest := NewPasswordModifyRequest("", "OldPassword", "")
	passwordModifyResponse, err := l.PasswordModify(passwordModifyRequest)
	if err != nil {
		log.Fatalf("Password could not be changed: %s", err.Error())
	}

	generatedPassword := passwordModifyResponse.GeneratedPassword
	log.Printf("Generated password: %s\n", generatedPassword)
}

func ExampleConn_PasswordModify_setNewPassword() {
	l, err := Dial("tcp", fmt.Sprintf("%s:%d", "ldap.example.com", 389))
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	err = l.Bind("cn=user,dc=example,dc=com", "password")
	if err != nil {
		log.Fatal(err)
	}

	passwordModifyRequest := NewPasswordModifyRequest("", "OldPassword", "NewPassword")
	_, err = l.PasswordModify(passwordModifyRequest)

	if err != nil {
		log.Fatalf("Password could not be changed: %s", err.Error())
	}
}

func ExampleConn_Modify() {
	l, err := Dial("tcp", fmt.Sprintf("%s:%d", "ldap.example.com", 389))
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	// Add a description, and replace the mail attributes
	modify := NewModifyRequest("cn=user,dc=example,dc=com", nil)
	modify.Add("description", []string{"An example user"})
	modify.Replace("mail", []string{"user@example.org"})

	err = l.Modify(modify)
	if err != nil {
		log.Fatal(err)
	}
}

// Example User Authentication shows how a typical application can verify a login attempt
func Example_userAuthentication() {
	// The username and password we want to check
	username := "someuser"
	password := "userpassword"

	bindusername := "readonly"
	bindpassword := "password"

	l, err := Dial("tcp", fmt.Sprintf("%s:%d", "ldap.example.com", 389))
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	// Reconnect with TLS
	err = l.StartTLS(&tls.Config{InsecureSkipVerify: true})
	if err != nil {
		log.Fatal(err)
	}

	// First bind with a read only user
	err = l.Bind(bindusername, bindpassword)
	if err != nil {
		log.Fatal(err)
	}

	// Search for the given username
	searchRequest := NewSearchRequest(
		"dc=example,dc=com",
		ScopeWholeSubtree, NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(objectClass=organizationalPerson)(uid=%s))", username),
		[]string{"dn"},
		nil,
	)

	sr, err := l.Search(searchRequest)
	if err != nil {
		log.Fatal(err)
	}

	if len(sr.Entries) != 1 {
		log.Fatal("User does not exist or too many entries returned")
	}

	userdn := sr.Entries[0].DN

	// Bind as the user to verify their password
	err = l.Bind(userdn, password)
	if err != nil {
		log.Fatal(err)
	}

	// Rebind as the read only user for any further queries
	err = l.Bind(bindusername, bindpassword)
	if err != nil {
		log.Fatal(err)
	}
}

func Example_beherappolicy() {
	l, err := Dial("tcp", fmt.Sprintf("%s:%d", "ldap.example.com", 389))
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	controls := []Control{}
	controls = append(controls, NewControlBeheraPasswordPolicy())
	bindRequest := NewSimpleBindRequest("cn=admin,dc=example,dc=com", "password", controls)

	r, err := l.SimpleBind(bindRequest)
	ppolicyControl := FindControl(r.Controls, ControlTypeBeheraPasswordPolicy)

	var ppolicy *ControlBeheraPasswordPolicy
	if ppolicyControl != nil {
		ppolicy = ppolicyControl.(*ControlBeheraPasswordPolicy)
	} else {
		log.Printf("ppolicyControl response not available.\n")
	}
	if err != nil {
		errStr := "ERROR: Cannot bind: " + err.Error()
		if ppolicy != nil && ppolicy.Error >= 0 {
			errStr += ":" + ppolicy.ErrorString
		}
		log.Print(errStr)
	} else {
		logStr := "Login Ok"
		if ppolicy != nil {
			if ppolicy.Expire >= 0 {
				logStr += fmt.Sprintf(". Password expires in %d seconds\n", ppolicy.Expire)
			} else if ppolicy.Grace >= 0 {
				logStr += fmt.Sprintf(". Password expired, %d grace logins remain\n", ppolicy.Grace)
			}
		}
		log.Print(logStr)
	}
}

func Example_vchuppolicy() {
	l, err := Dial("tcp", fmt.Sprintf("%s:%d", "ldap.example.com", 389))
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()
	l.Debug = true

	bindRequest := NewSimpleBindRequest("cn=admin,dc=example,dc=com", "password", nil)

	r, err := l.SimpleBind(bindRequest)

	passwordMustChangeControl := FindControl(r.Controls, ControlTypeVChuPasswordMustChange)
	var passwordMustChange *ControlVChuPasswordMustChange
	if passwordMustChangeControl != nil {
		passwordMustChange = passwordMustChangeControl.(*ControlVChuPasswordMustChange)
	}

	if passwordMustChange != nil && passwordMustChange.MustChange {
		log.Printf("Password Must be changed.\n")
	}

	passwordWarningControl := FindControl(r.Controls, ControlTypeVChuPasswordWarning)

	var passwordWarning *ControlVChuPasswordWarning
	if passwordWarningControl != nil {
		passwordWarning = passwordWarningControl.(*ControlVChuPasswordWarning)
	} else {
		log.Printf("ppolicyControl response not available.\n")
	}
	if err != nil {
		log.Print("ERROR: Cannot bind: " + err.Error())
	} else {
		logStr := "Login Ok"
		if passwordWarning != nil {
			if passwordWarning.Expire >= 0 {
				logStr += fmt.Sprintf(". Password expires in %d seconds\n", passwordWarning.Expire)
			}
		}
		log.Print(logStr)
	}
}

// This example demonstrates how to use ControlPaging to manually execute a
// paginated search request instead of using SearchWithPaging.
func ExampleControlPaging_manualPaging() {
	conn, err := Dial("tcp", fmt.Sprintf("%s:%d", "ldap.example.com", 389))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	var pageSize uint32 = 32
	searchBase := "dc=example,dc=com"
	filter := "(objectClass=group)"
	pagingControl := NewControlPaging(pageSize)
	attributes := []string{}
	controls := []Control{pagingControl}

	for {
		request := NewSearchRequest(searchBase, ScopeWholeSubtree, DerefAlways, 0, 0, false, filter, attributes, controls)
		response, err := conn.Search(request)
		if err != nil {
			log.Fatalf("Failed to execute search request: %s", err.Error())
		}

		// [do something with the response entries]

		// In order to prepare the next request, we check if the response
		// contains another ControlPaging object and a not-empty cookie and
		// copy that cookie into our pagingControl object:
		updatedControl := FindControl(response.Controls, ControlTypePaging)
		if ctrl, ok := updatedControl.(*ControlPaging); ctrl != nil && ok && len(ctrl.Cookie) != 0 {
			pagingControl.SetCookie(ctrl.Cookie)
			continue
		}
		// If no new paging information is available or the cookie is empty, we
		// are done with the pagination.
		break
	}
}
