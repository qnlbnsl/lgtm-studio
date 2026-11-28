package projects_test

import "testing"

var uncleanReportPaths = []struct {
	name string
	path string
}{
	{"DoubledSeparator", "/workspace//checkout"},
	{"DotComponent", "/workspace/./checkout"},
	{"ParentComponent", "/workspace/subfolder/../checkout"},
	{"TrailingSeparator", "/workspace/checkout/"},
}

func TestProjectReportResolvedPathSpelling(t *testing.T) {
	f := newReportFixture(t, reportControls{})
	var mark uint64
	t.Run("Clean", func(t *testing.T) {
		op := f.admit(t, "inspect clean checkout", false)
		mark++
		sendFolderReport(t, f.stream, mark, op.facts())
		f.requireAck(t, f.stream, f.machine, mark)
		f.requireInspect(t, op, expectedReportFacts())
	})
	for _, tt := range uncleanReportPaths {
		t.Run(tt.name, func(t *testing.T) {
			op := f.admit(t, "inspect "+tt.name, false)
			before := f.row(t, op)
			seen := op.facts()
			seen.GetFacts().ResolvedPath = tt.path
			mark++
			sendFolderReport(t, f.stream, mark, seen)
			f.requireAck(t, f.stream, f.machine, mark)
			f.requireUnchanged(t, op, before)

			mark++
			sendFolderReport(t, f.stream, mark, op.facts())
			f.requireAck(t, f.stream, f.machine, mark)
			f.requireInspect(t, op, expectedReportFacts())
			f.requireNoMembership(t)
		})
	}
}

func TestProjectReportUncleanPathSkipsAttachment(t *testing.T) {
	for _, tt := range uncleanReportPaths {
		t.Run(tt.name, func(t *testing.T) {
			f := newReportFixture(t, reportControls{})
			op := f.admit(t, "add checkout", true)
			before := f.row(t, op)
			// The attachment stub fails before an insert. The trigger also fails
			// the write if attachment reaches the insert once item 5 is built.
			f.exec(t, `
CREATE FUNCTION fixture_reject_review_attachment() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'The fixture rejects repository creation.'; END
$$;
CREATE TRIGGER fixture_reject_review_attachment BEFORE INSERT ON repositories
FOR EACH ROW EXECUTE FUNCTION fixture_reject_review_attachment();`)
			seen := op.facts()
			seen.GetFacts().ResolvedPath = tt.path
			sendFolderReport(t, f.stream, 1, seen)
			f.requireAck(t, f.stream, f.machine, 1)
			f.requireUnchanged(t, op, before)
			f.requireNoMembership(t)
		})
	}
}

func TestProjectReportInspectPreservesOriginPortSpelling(t *testing.T) {
	const origin = "ssh://example.test:02222/checkout.git"
	f := newReportFixture(t, reportControls{})
	op := f.admit(t, "inspect origin port spelling", false)
	seen := op.facts()
	seen.GetFacts().Remote = new(origin)
	sendFolderReport(t, f.stream, 1, seen)
	f.requireAck(t, f.stream, f.machine, 1)
	want := expectedReportFacts()
	want.Remote = new(origin)
	f.requireInspect(t, op, want)
}
