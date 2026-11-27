package contracts_test

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

const projectsSource = "proto/lgtm/v1/projects.proto"

func TestProjectsMatchItsDescription(t *testing.T) {
	matchDescription(t, "testdata/golden/projects.txt")
	file, err := protoregistry.GlobalFiles.FindFileByPath("lgtm/v1/projects.proto")
	if err != nil {
		t.Fatal("the repository service has no generated projects.proto descriptor")
	}
	if file.Package() != "lgtm.v1" || file.Messages().Len() != 10 || file.Services().Len() != 1 {
		t.Errorf("projects.proto has package %s, %d messages and %d services, want lgtm.v1, ten messages and one service",
			file.Package(), file.Messages().Len(), file.Services().Len())
	}
	if service := file.Services().ByName("ProjectService"); service == nil || service.Methods().Len() != 3 {
		t.Error("ProjectService must declare exactly three methods")
	}
}

func projectSourceLines(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(projectsSource)
	if os.IsNotExist(err) {
		t.Fatal("the ProjectService declarations and contract clauses are missing")
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(string(raw), "\n")
}

func TestProjectFolderCallsRequireRequestKeys(t *testing.T) {
	lines := projectSourceLines(t)
	service := normalize(commentBefore(t, lines, "service ProjectService {"))
	requireWrittenRules(t, "ProjectService", service, [][2]string{
		{"all calls require a session at entry", `(every|all) call[^.]*session[^.]*entry|session[^.]*(required|require)[^.]*(every|all) call[^.]*entry`},
	})
	for _, method := range []string{"InspectFolder", "AddRepository"} {
		t.Run(method, func(t *testing.T) {
			comment := normalize(commentBefore(t, lines, "rpc "+method+"("))
			requireWrittenRules(t, method, comment, [][2]string{
				{"the call requires Lgtm-Request-Key", `requir[^.]*lgtm-request-key|lgtm-request-key[^.]*requir`},
			})
			if regexp.MustCompile(`(does not require|never requires|optional)[^.]*lgtm-request-key|lgtm-request-key[^.]*(is optional|is not required)`).MatchString(strings.ToLower(comment)) {
				t.Error("the method comment makes the required request key optional")
			}
		})
	}
	requireWrittenRules(t, "the folder operation rules", normalize(lines), [][2]string{
		{"the header is exactly one nonempty request key under the store text rules and 256-byte limit",
			`(one|exactly one) non[- ]?empty[^.]*(lgtm-request-key|request key)[^.]*store[^.]*text[^.]*256[- ]byte`},
		{"the request identity includes caller, procedure, machine, exact typed path and optional repository choice",
			`request identity[^.]*caller[^.]*procedure[^.]*machine[^.]*exact typed path[^.]*optional repository`},
		{"a present empty target is invalid", `present empty (target|repository_id)[^.]*invalid`},
		{"a fresh request stores operation and command together", `fresh request[^.]*stor[^.]*operation[^.]*command[^.]*(together|same write)`},
		{"a fresh disconnected operation is unavailable", `fresh[^.]*disconnected[^.]*unavailable`},
		{"the command expires after 20 seconds and the caller waits at most 25 seconds outside the write",
			`command[^.]*expir[^.]*20 seconds[^.]*(at most|up to) 25 seconds[^.]*outside[^.]*write`},
		{"an accepted retry finds the original operation before the availability check",
			`(retry|replay)[^.]*(original|same) operation[^.]*before[^.]*availability`},
		{"a different request using the same caller key stays a conflict throughout replay",
			`different request[^.]*(same|caller)[^.]*key[^.]*already_exists[^.]*throughout replay|conflict[^.]*caller[^.]*key[^.]*refus[^.]*throughout replay`},
		{"once admitted, a replay keeps the original result if retirement races the read",
			`admitted replay[^.]*(keep|gets|returns|read)[^.]*(original|saved) result[^.]*retirement`},
		{"retirement winning the race applies fresh-request rules", `retirement wins[^.]*fresh[- ]request rules`},
	})
}

func TestRepositoryReplyCanJoinItsWatchRecord(t *testing.T) {
	const repoID = "0193a2b4-0000-7000-8000-000000000001"
	const folderID = "0193a2b4-0000-7000-8000-000000000002"
	const clientID = "0193a2b4-0000-7000-8000-000000000003"
	const otherClientID = "0193a2b4-0000-7000-8000-000000000004"
	const otherFolderID = "0193a2b4-0000-7000-8000-000000000005"
	const path = "/work/checkout"
	const remoteURL = "https://127.0.0.1/origin"
	folderText := fmt.Sprintf("id: %q client_id: %q resolved_path: %q", folderID, clientID, path)
	otherFolderText := fmt.Sprintf("id: %q client_id: %q resolved_path: %q", otherFolderID, otherClientID, path)

	for _, tc := range []struct {
		name, origin      string
		present, withheld bool
	}{
		{"present", fmt.Sprintf("remote: %q", remoteURL), true, false},
		{"absent", "", false, false},
		{"withheld", "remote_omitted: true", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repoText := fmt.Sprintf("id: %q name: \"checkout\" main_branch: \"main\" first_commit: \"abc123\" folders: {%s} folders: {%s} %s",
				repoID, otherFolderText, folderText, tc.origin)
			repo := fixtureMessage(t, "lgtm.v1.Repository", repoText)
			folder := fixtureMessage(t, "lgtm.v1.RepositoryFolder", folderText)
			// Encode the two reply tags independently so a symmetric change of the descriptor cannot hide a tag change.
			repoWire, err := proto.Marshal(repo.Interface())
			if err != nil {
				t.Fatal(err)
			}
			folderWire, err := proto.Marshal(folder.Interface())
			if err != nil {
				t.Fatal(err)
			}
			reply := newMessage(t, "lgtm.v1.AddRepositoryResponse")
			if err := proto.Unmarshal(wireBytes(wireBytes(nil, 1, repoWire), 2, folderWire), reply.Interface()); err != nil {
				t.Fatal(err)
			}
			reply = wireRoundTrip(t, reply)
			replyRepo := requiredMessage(t, reply, "repository")
			replyFolder := requiredMessage(t, reply, "folder")

			snapshot := wireRoundTrip(t, fixtureMessage(t, "lgtm.v1.WatchRepositoriesResponse",
				fmt.Sprintf("all: {repositories: {%s}} position: {value: 3}", repoText)))
			saved := wireRoundTrip(t, fixtureMessage(t, "lgtm.v1.WatchRepositoriesResponse",
				fmt.Sprintf("saved: {%s} position: {value: 9}", repoText)))
			repos := requiredMessage(t, snapshot, "all")
			list := repos.Get(messageField(t, repos, "repositories")).List()
			if list.Len() != 1 {
				t.Fatalf("the snapshot must contain one repository, got %d", list.Len())
			}
			for _, frame := range []struct {
				message  protoreflect.Message
				position uint64
			}{{snapshot, 3}, {saved, 9}} {
				position := requiredMessage(t, frame.message, "position")
				if got := position.Get(messageField(t, position, "value")).Uint(); got != frame.position {
					t.Errorf("watch position = %d, want %d", got, frame.position)
				}
			}
			inspect := wireRoundTrip(t, fixtureMessage(t, "lgtm.v1.InspectFolderResponse",
				fmt.Sprintf("facts: {resolved_path: %q name: \"checkout\" main_branch: \"main\" clean: false %s}", path, tc.origin)))
			facts := requiredMessage(t, inspect, "facts")
			if facts.Has(messageField(t, facts, "checked_out_branch")) || facts.Get(messageField(t, facts, "clean")).Bool() {
				t.Error("folder inspection must preserve a dirty detached checkout")
			}

			for _, record := range []protoreflect.Message{replyRepo, list.Get(0).Message(), requiredMessage(t, saved, "saved"), facts} {
				remote := messageField(t, record, "remote")
				if record.Has(remote) != tc.present || record.Get(messageField(t, record, "remote_omitted")).Bool() != tc.withheld ||
					(tc.present && record.Get(remote).String() != remoteURL) {
					t.Errorf("%s lost the %s origin", record.Descriptor().Name(), tc.name)
				}
			}
			for _, record := range []protoreflect.Message{replyRepo, list.Get(0).Message(), requiredMessage(t, saved, "saved")} {
				id := messageField(t, record, "id")
				if got := record.Get(id).String(); got != repoID || got != replyRepo.Get(id).String() {
					t.Errorf("repository identity = %q, want %q in reply and watch", got, repoID)
				}
				folders := record.Get(messageField(t, record, "folders")).List()
				if folders.Len() != 2 {
					t.Fatalf("the repository must carry the full two-folder list, got %d", folders.Len())
				}
				matches := 0
				for i := 0; i < folders.Len(); i++ {
					stored := folders.Get(i).Message()
					if proto.Equal(stored.Interface(), replyFolder.Interface()) {
						matches++
					}
				}
				if matches != 1 {
					t.Errorf("the add reply's folder matched %d watch folders, want one", matches)
				}
			}
			for _, pair := range [][2]string{{"id", folderID}, {"client_id", clientID}, {"resolved_path", path}} {
				if got := replyFolder.Get(messageField(t, replyFolder, pair[0])).String(); got != pair[1] {
					t.Errorf("add reply folder %s = %q, want %q", pair[0], got, pair[1])
				}
			}
		})
	}
}

func requiredMessage(t *testing.T, parent protoreflect.Message, name string) protoreflect.Message {
	t.Helper()
	field := messageField(t, parent, name)
	if field.Message() == nil || field.IsList() || !parent.Has(field) {
		t.Fatalf("%s.%s must carry a singular message", parent.Descriptor().Name(), name)
	}
	return parent.Get(field).Message()
}

func TestProjectRefusalRulesAreWritten(t *testing.T) {
	lines := projectSourceLines(t)
	// Ignore source wrapping and table separators while keeping the condition, code and sentence together.
	text := normalize(lines)
	text = strings.TrimSpace(spaces.ReplaceAllString(strings.NewReplacer("`", "", "|", " ").Replace(text), " "))
	for _, row := range []struct{ condition, code, sentence string }{
		{"Session missing or expired at entry", "unauthenticated", "Sign in to continue."},
		{"Session ends during a watch", "unauthenticated", "Sign in again to reopen this watch."},
		{"Watch falls behind", "aborted", "The watch fell behind and must be reopened."},
		{"Malformed JSON body on a protected call", "invalid_argument", "The request body is invalid."},
		{"Machine is disconnected when starting a new operation", "unavailable", "The machine is not connected."},
		{"Invalid machine or repository identifier, or invalid typed path", "invalid_argument", "The machine, repository, or folder path is invalid."},
		{"Invalid or missing request key", "invalid_argument", "The request key is invalid."},
		{"Same key for a different request", "already_exists", "That request key was already used for a different request."},
		{"Unknown machine", "not_found", "The machine was not found."},
		{"Unknown explicit repository at attachment, even if the folder is already saved elsewhere", "not_found", "The repository was not found."},
		{"FOLDER_REFUSAL_NOT_A_FOLDER", "failed_precondition", "The path is not a folder."},
		{"FOLDER_REFUSAL_NOT_A_GIT_REPOSITORY", "failed_precondition", "The folder is not a Git repository."},
		{"FOLDER_REFUSAL_NOT_READABLE", "failed_precondition", "The folder cannot be read."},
		{"FOLDER_REFUSAL_PATH_NOT_USABLE", "failed_precondition", "The folder path cannot be used."},
		{"Folder does not match the selected repository", "failed_precondition", "The folder does not belong to the selected repository."},
		{"Folder is already attached to another repository and the explicit target exists", "already_exists", "The folder is already saved for another repository."},
		{"Call wait ends or done or failed arrives without a report", "deadline_exceeded", "The machine did not answer the folder request."},
		{"Expired or unsupported without a report", "deadline_exceeded", "The machine did not take the folder command."},
		{"Interrupted or timed out without a report", "deadline_exceeded", "The outcome of the folder request is unknown."},
		{"The machine is removed before the operation finishes", "not_found", "The machine was removed."},
	} {
		pattern := "(?i:" + regexp.QuoteMeta(row.condition) + ")[ :–—-]+" + row.code + "[ :–—-]+[\x22]?" + regexp.QuoteMeta(row.sentence)
		if !regexp.MustCompile(pattern).MatchString(text) {
			t.Errorf("projects.proto must pair %q with %s and %q", row.condition, row.code, row.sentence)
		}
	}
	requireWrittenRules(t, "the attachment rules", text, [][2]string{
		{"an unknown explicit target takes precedence for both saved and unsaved folders",
			`unknown explicit target[^.]*not_found[^.]*both[^.]*saved and (unsaved|new) (resolved )?folder|unknown explicit target[^.]*(before|takes precedence)[^.]*existing-folder conflict`},
		{"a saved folder is returned for an omitted or matching target even if the origin changed",
			`return[^.]*existing folder[^.]*omitted or matching target[^.]*origin changed`},
		{"a new attachment needs equal present origins or equal present first commits when an origin is known absent",
			`present origins[^.]*equal present first commits[^.]*origin[^.]*known absent|matching present origins[^.]*equal present first commits[^.]*known absent`},
		{"withholding an origin is no evidence of absence", `withhold[^.]*no evidence of absence|withheld[^.]*does not[^.]*absence`},
		{"account-relative Secure Shell paths are withheld and not compared",
			`account-relative[^.]*(secure shell|ssh)[^.]*(withh|remote_omitted)[^.]*(never|not) compar`},
	})
	requireWrittenRules(t, "WatchRepositories", normalize(commentBefore(t, lines, "rpc WatchRepositories(")), [][2]string{
		{"the session is required for the duration of the watch", `session[^.]*(life|duration|throughout)[^.]*watch|session[^.]*throughout`},
		{"a replacement snapshot comes first", `replacement snapshot[^.]*first|first[^.]*replacement snapshot`},
		{"full saved records or removed identifiers follow", `full saved repositor[^.]*folder[^.]*removed identif`},
		{"the shared FeedPosition rules apply", `(shared|common)[^.]*feedposition|feedposition[^.]*(shared|common)`},
		{"an aborted watch is reopened", `aborted[^.]*reopen|reopen[^.]*aborted`},
	})
	requireWrittenRules(t, "folder operation settlement", text, [][2]string{
		{"the first valid report or terminal outcome ends an open operation",
			`first valid report[^.]*terminal outcome[^.]*ends?[^.]*open operation`},
		{"a valid report applied before settlement in the same write wins",
			`valid report[^.]*before settlement[^.]*same write[^.]*wins`},
		{"an eligible late report advances the mark and is acknowledged",
			`eligible late report[^.]*advanc[^.]*mark[^.]*acknowledg`},
		{"the add reply holds the saved identities and its folder belongs to its repository",
			`(add reply|addrepositoryresponse)[^.]*saved identit[^.]*folder[^.]*belong[^.]*repositor`},
		{"the repository and folder persist together or neither persists even after the call ends",
			`(both or neither|repository and folder[^.]*same write)[^.]*after the call ends`},
	})
}
