# References for the open points of the owner's review of 2026-11-23

The owner read every screen on 2026-11-23, and the review is issue 1572.
Four of its open points are about where things sit, not how they look.
This file holds what shipped products do for each of the four.
The screens come from Mobbin, a library of screens of shipped products.
A product is described only from a screen that was looked at.

The older research on each screen is in `docs/research/2026-11-20-screen-references/`.

## 1. A project's own button, and a place for repositories

Today the plus beside "Repositories" in the sidebar adds a project.

- **Asana** names each sidebar group after what its plus adds. The group "Projects" has a plus that creates a project. A page "Browse projects" lists them all, with one button, "Create project". [Screen](https://mobbin.com/screens/b0a86e2e-051a-4025-8c40-531067c85c22)
- **Sentry** keeps repositories out of the sidebar. A settings page lists the connected repositories, one row each with a remove button, under one button, "Add Repository". [Screen](https://mobbin.com/screens/da54b970-6ad0-4355-8d49-d9a84e37d47c)
- **Vercel** connects a repository inside a project's settings. The list has a search field and one "Connect" button on each row. [Screen](https://mobbin.com/screens/b394a591-086d-4e82-a0d0-71e855ef3b25)
- **Todoist** shows a workspace as a page. The page lists the workspace's projects in a table, and one "Add" button offers a project or a folder. [Screen](https://mobbin.com/screens/4c39b647-967e-49a8-8907-74ab0bf6a75f)

Options for LGTM Studio:

1. The sidebar group is named "Projects", and its plus adds a project. Repositories move to a page of the service settings, as in Sentry.
2. The group stays "Repositories". Each repository's row gets its own plus for a project, and the group's plus adds a repository.
3. A repository is a page, as in Todoist. The page holds its projects, its board and its roadmap, and "New project" is the page's one button.

Option 3 fits the owner's idea that the repository level is for project management.

## 2. A home built on what needs attention

- **Graphite** groups its inbox by what the reader must do: "Needs your review", "Returned to you", "Approved", "Drafts". Each group has a count and folds. An empty group stays as one line. [Screen](https://mobbin.com/screens/a83d8ccd-c487-4e28-8edc-967742f33219)
- **GitLab** has two tabs, "To Do" and "Done". A row is one line of text with its age at the right, and one button marks it done. [Screen](https://mobbin.com/screens/24eba772-b1c6-42f2-82b4-0a329fb5ba59)
- **Lemni** puts the list on the left and the open item beside it. "Approve" and "Decline" sit at the foot of the item, so the reader answers without leaving the inbox. [Screen](https://mobbin.com/screens/31c9c03a-dff3-4ab4-9710-b2990d13adbe)
- **Plane** uses the same split. The inbox is a narrow list, and the item opens in full beside it. [Screen](https://mobbin.com/screens/849ec3f0-a1f0-4699-85eb-f895f7330036)

Options for LGTM Studio:

1. Home is the Questions view for every project at once: the list on one side and the open question beside it, as in Lemni and Plane.
2. Home groups what waits by what the owner must do: answer, approve, merge. Each group folds, as in Graphite.
3. Home stays a single list, cut to one line a row, as in GitLab.

## 3. Showing the stage of a project

Today the stage is at the right end of the row of views.

- **Asana** puts the state beside the project's name, as a small word with a dot. [Screen](https://mobbin.com/screens/b4d2dcf6-d4b0-4513-a0e0-8ac232dc6b5c)
- **Linear** puts the state first in a line of properties under the title. [Screen](https://mobbin.com/screens/186d053a-bfff-4742-8333-06cd35a58e5b)
- **Bonsai** puts the stage at the right of the title as a button, and the button opens the list of stages. [Screen](https://mobbin.com/screens/9c4cebe3-676b-46cc-9c91-f85d78338675)
- **Cofounder** shows a strip of steps along the foot of the page, with "3/8" beside its name. [Screen](https://mobbin.com/screens/f48f81f0-d399-4b29-a733-b3b942b6c7e9)

Options for LGTM Studio:

1. The stage follows the project's name in the top bar, as in Asana: "lgtm-studio › Agent console · Build". The eye lands on the name first.
2. The stage is a strip of eight small steps under the row of views, as in Cofounder. It shows the past and the future too, and it costs a row.
3. The stage stays where it is and gets more weight.

## 4. Where the branch is chosen

Today a branch menu sits in the top bar, beside the bell.

- **Cursor** asks for the repository and the branch where the work is started, above the message box. The top bar has neither. [Screen](https://mobbin.com/screens/9042dc24-874c-4f95-821e-b9b63972426e)
- **Devin** shows the repository's name and its branch together, as one path at the left of the top bar. [Screen](https://mobbin.com/screens/96150d13-2f37-48c5-bee1-d195a41bf195)

Options for LGTM Studio:

1. The branch joins the path at the left of the top bar, as in Devin: repository, project, branch.
2. The branch leaves the top bar. It is shown and changed in the views that read it: Files, Git and Terminal.

## The searches

Four searches were run on Mobbin on 2026-11-23, for web screens.
The search for a sidebar of repositories with projects under each found no product that nests the two.
The search for an approval inbox was strong.
