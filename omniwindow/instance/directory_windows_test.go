package instance

import (
	"golang.org/x/sys/windows"
	"testing"
)

func TestPrivateDirectoryACL(t *testing.T) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sid := user.User.Sid.String()
	for _, test := range []struct {
		sddl  string
		valid bool
	}{
		{"O:" + sid + "D:P(A;OICI;FA;;;" + sid + ")(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)", true},
		{"O:" + sid + "D:P(A;;FA;;;WD)", false},
		{"O:" + sid + "D:P(A;OIIO;GR;;;WD)(A;;FA;;;" + sid + ")", false},
		{"O:SYD:P(A;;FA;;;" + sid + ")", false},
	} {
		sd, err := windows.SecurityDescriptorFromString(test.sddl)
		if err != nil {
			t.Fatal(err)
		}
		err = validateDirectoryACL(sd, user.User.Sid)
		if (err == nil) != test.valid {
			t.Fatalf("%q: %v", test.sddl, err)
		}
	}
}
