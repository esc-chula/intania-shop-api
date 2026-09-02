package usecases_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/repositories"
	"github.com/esc-chula/intania-shop-api/internal/usecases"
)

type projectWriterStub struct {
	created   models.ProjectInput
	updated   models.ProjectInput
	updatedID int64
	deletedID int64
	today     models.Date
	err       error
}

func (stub *projectWriterStub) Create(_ context.Context, today models.Date, input models.ProjectInput) (models.Project, error) {
	stub.today, stub.created = today, input
	return models.Project{ProjectID: 7, Name: *input.Name}, stub.err
}

func (stub *projectWriterStub) Update(_ context.Context, today models.Date, projectID int64, input models.ProjectInput) (models.Project, error) {
	stub.today, stub.updatedID, stub.updated = today, projectID, input
	return models.Project{ProjectID: projectID, Name: *input.Name}, stub.err
}

func (stub *projectWriterStub) Delete(_ context.Context, projectID int64) error {
	stub.deletedID = projectID
	return stub.err
}

func projectInput(t *testing.T, name string, startDate, endDate string) models.ProjectInput {
	t.Helper()

	input := models.ProjectInput{Name: &name}
	if startDate != "" {
		input.StartDate = parsedDate(t, startDate)
	}
	if endDate != "" {
		input.EndDate = parsedDate(t, endDate)
	}
	return input
}

func parsedDate(t *testing.T, value string) *models.Date {
	t.Helper()

	date, err := models.ParseDate(value)
	if err != nil {
		t.Fatalf("parse date %q: %v", value, err)
	}
	return &date
}

// Every rejected payload has to name the field that was wrong, so that the
// client can show the message without translating an error code.
func TestProjectAdminServiceRejectsInvalidInput(t *testing.T) {
	longDescription := strings.Repeat("d", models.ProjectDescriptionMaxLength+1)

	tests := []struct {
		name  string
		input models.ProjectInput
		want  string
	}{
		{name: "missing name", input: models.ProjectInput{StartDate: parsedDate(t, "2026-09-05"), EndDate: parsedDate(t, "2026-09-05")}, want: "name"},
		{name: "blank name", input: projectInput(t, "   ", "2026-09-05", "2026-09-05"), want: "name"},
		{name: "name too long", input: projectInput(t, strings.Repeat("n", models.ProjectNameMaxLength+1), "2026-09-05", "2026-09-05"), want: "name"},
		{name: "missing start date", input: projectInput(t, "Engineering Fair", "", "2026-09-05"), want: "start date"},
		{name: "missing end date", input: projectInput(t, "Engineering Fair", "2026-09-05", ""), want: "end date"},
		{name: "end date before start date", input: projectInput(t, "Engineering Fair", "2026-09-07", "2026-09-05"), want: "end date"},
		{name: "description too long", input: describedProject(t, &longDescription), want: "description"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stub := &projectWriterStub{}
			service := usecases.NewProjectService(nil, stub)

			_, err := service.Create(context.Background(), test.input)
			assertProjectValidationError(t, err, test.want)

			// The same rules have to reject the same payload on update.
			if _, err = service.Update(context.Background(), 7, test.input); err == nil {
				t.Fatal("update accepted an invalid payload")
			}
			if stub.created.Name != nil || stub.updated.Name != nil {
				t.Error("an invalid payload reached the repository")
			}
		})
	}
}

func assertProjectValidationError(t *testing.T, err error, wantField string) {
	t.Helper()

	var validation usecases.ProjectValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("err=%v, want a ProjectValidationError", err)
	}
	if !strings.Contains(strings.ToLower(validation.Message), wantField) {
		t.Errorf("message %q does not mention %q", validation.Message, wantField)
	}
}

func TestProjectAdminServiceCreatesSingleDayAndMultiDayProjects(t *testing.T) {
	tests := []struct {
		name               string
		startDate, endDate string
	}{
		{name: "single day", startDate: "2026-09-05", endDate: "2026-09-05"},
		{name: "multi day", startDate: "2026-09-05", endDate: "2026-09-07"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stub := &projectWriterStub{}
			project, err := usecases.NewProjectService(nil, stub).
				Create(context.Background(), projectInput(t, "Engineering Fair", test.startDate, test.endDate))
			if err != nil {
				t.Fatalf("create project: %v", err)
			}
			if project.ProjectID != 7 {
				t.Errorf("project_id=%d want 7", project.ProjectID)
			}
			if stub.created.StartDate.String() != test.startDate || stub.created.EndDate.String() != test.endDate {
				t.Errorf("persisted %s..%s, want %s..%s",
					stub.created.StartDate, stub.created.EndDate, test.startDate, test.endDate)
			}
		})
	}
}

// The stored name is the trimmed one, so that " Fair " and "Fair" cannot become
// two projects that look identical in the listing.
func TestProjectAdminServiceTrimsTheName(t *testing.T) {
	stub := &projectWriterStub{}
	if _, err := usecases.NewProjectService(nil, stub).
		Create(context.Background(), projectInput(t, "  Engineering Fair  ", "2026-09-05", "2026-09-07")); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if *stub.created.Name != "Engineering Fair" {
		t.Errorf("persisted name=%q want %q", *stub.created.Name, "Engineering Fair")
	}
}

// Status is derived from the Bangkok calendar date, so the write path has to
// resolve "today" the same way the query path does.
func TestProjectAdminServiceResolvesTodayInBangkok(t *testing.T) {
	stub := &projectWriterStub{}
	if _, err := usecases.NewProjectService(nil, stub).
		Create(context.Background(), projectInput(t, "Engineering Fair", "2026-09-05", "2026-09-07")); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if want := models.TodayInBangkok(time.Now()); stub.today != want {
		t.Errorf("today=%s want %s", stub.today, want)
	}
}

func TestProjectAdminServiceRejectsNonPositiveIDs(t *testing.T) {
	stub := &projectWriterStub{}
	service := usecases.NewProjectService(nil, stub)
	input := projectInput(t, "Engineering Fair", "2026-09-05", "2026-09-07")

	if _, err := service.Update(context.Background(), 0, input); !errors.Is(err, usecases.ErrInvalidProjectID) {
		t.Errorf("update err=%v, want ErrInvalidProjectID", err)
	}
	if err := service.Delete(context.Background(), -1); !errors.Is(err, usecases.ErrInvalidProjectID) {
		t.Errorf("delete err=%v, want ErrInvalidProjectID", err)
	}
	if stub.updatedID != 0 || stub.deletedID != 0 {
		t.Error("an invalid ID reached the repository")
	}
}

// The transport maps repository failures onto status codes, so the use case has
// to keep them inspectable through its own wrapping.
func TestProjectAdminServicePreservesRepositoryErrors(t *testing.T) {
	stub := &projectWriterStub{err: repositories.ErrProjectNotFound}
	service := usecases.NewProjectService(nil, stub)

	if _, err := service.Update(context.Background(), 7, projectInput(t, "Engineering Fair", "2026-09-05", "2026-09-07")); !errors.Is(err, repositories.ErrProjectNotFound) {
		t.Errorf("update err=%v, want ErrProjectNotFound", err)
	}
	if err := service.Delete(context.Background(), 7); !errors.Is(err, repositories.ErrProjectNotFound) {
		t.Errorf("delete err=%v, want ErrProjectNotFound", err)
	}
}

// describedProject is an otherwise valid payload carrying a description.
func describedProject(t *testing.T, description *string) models.ProjectInput {
	t.Helper()

	input := projectInput(t, "Engineering Fair", "2026-09-05", "2026-09-05")
	input.Description = description
	return input
}
