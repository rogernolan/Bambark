package notification

import "testing"

func TestFromBambuddyMapsTitleAndMessage(t *testing.T) {
	got, err := FromBambuddy(BambuddyPayload{Title: "  Print failed ", Message: "  Nozzle error  "})
	if err != nil {
		t.Fatalf("FromBambuddy() error = %v", err)
	}
	if got != (Notification{Title: "Print failed", Body: "Nozzle error"}) {
		t.Fatalf("notification = %#v", got)
	}
}

func TestFromBambuddyAcceptsBambuddyTestFields(t *testing.T) {
	got, err := FromBambuddy(BambuddyPayload{
		Test: "Bambuddy Test",
		Push: "This is a test notification.",
	})
	if err != nil {
		t.Fatalf("FromBambuddy() error = %v", err)
	}
	if got != (Notification{Title: "Bambuddy Test", Body: "This is a test notification."}) {
		t.Fatalf("notification = %#v", got)
	}
}

func TestFromBambuddyRejectsBlankFields(t *testing.T) {
	for _, payload := range []BambuddyPayload{{Message: "body"}, {Title: "title"}, {Title: " ", Message: "body"}, {Title: "title", Message: "   "}} {
		if _, err := FromBambuddy(payload); err == nil {
			t.Errorf("FromBambuddy(%#v) error = nil", payload)
		}
	}
}
