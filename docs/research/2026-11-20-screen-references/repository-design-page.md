# Repository Design page: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

The source is `design/mock/js/screens/design.js`. The proposed standard behind its menus is `design/mock/feedback/2026-11-10-design-categories.md`.

- The page is one thin bar over a canvas that fills the rest of the window.
- The bar holds the repository's "Projects" and "Design" tabs, a category menu, an item menu, the design file's path with its age and who changed it, the zoom controls, and "Open in OpenPencil".
- The category menu has three entries: Screens, Components and Palette.
- The item menu depends on the category. Each entry shows a count of frames, components or tokens, such as "8 frames".
- Screens lists nine flows, such as "Sign in and setup", "Project" and "Code". A flow is drawn as a grid of frames at 1,440 by 900 pixels. Each row is one screen, and each column is one of its states.
- A frame with an address shows the mock's own screen inside it. A frame with no address is dashed and says which screen it opens from.
- A click on a frame zooms the canvas to that frame.
- Components lists an "Overview" and eight groups: Actions, Inputs, Navigation, Overlays, Status, Data, Agents and Code.
- A component that is in the design file shows a grid of its variants, with labels such as "Kind=Primary" and "State=Hover", and a flag "In the file". A component that is only in the code shows one preview and the flag "Only in code".
- Palette lists six items: Colour, Type, Space and density, Shape, Motion and Icons.
- Colour is a table grouped by Surfaces, Text, Action, States, Diff and Terminal. Each row has two swatches, the token name, the light value (Field), the dark value (Dusk), the Cascading Style Sheets (CSS) variable, and a flag: "Matches", "Retuned in code" or "Only in code".
- The palette values are read from `css/tokens.css` when the page opens, so the page compares the code with the design file.
- The zoom controls are minus, a percentage, plus and "Fit". Screens open zoomed out, at 20 percent or at the fit size when that is larger. The other categories open at 100 percent.
- A repository without a design file keeps the same structure, with empty tables and dashed placeholders. The bar's button reads "Create design file".
- On the live website the item menu holds one entry. Screens shows "No flows", Components is a table with a "View component" button, and Palette is a table of token names and values.

## What is planned and not built

- From the header of `design/mock/js/screens/design.js`: the two menus follow the Design lead's proposed standard, and "The owner has not approved it."
- From `design/mock/feedback/2026-11-10-design-categories.md`: the owner did not approve the categories on 2026-11-11, called them a good start, and decided to refine them through use.
- From the same file, "Pages": a page whose name matches no rule should be shown under Screens and marked "Not named to the standard". The mock does not show such a page.
- From `docs/features.md`, "10. Design and mock modes": "Recorded mock previews and OpenPencil" is decided, with "implementation pending". The engine starts and manages OpenPencil with access to the project's design folder only.
- From `docs/features.md`, "A full-size OpenPencil editor beside the chat": a maximize button lets the editor fill the page while the chat stays open. It was asked for on the Mocks screen, and it would touch this page's editor as well.
- From `design/DESIGN.md`, "Stages, stage teams, and mocks, decided on 2026-11-07": the repository Design tab has an "Edit mode" that shows OpenPencil's web editor in a frame. The page read today has a button that opens the editor and no mode switch.
- From the same section: OpenPencil "cannot save into the repository yet".
- From the same section: "A component that a project's approval promoted says which project it came from." No component on the page names a project.
- From the same section: "Converting the design into code belongs to the Build team", and the tool is built later.
- From `design/DESIGN.md`, "Libraries, set on 2026-11-10": the canvas is the mock's own small component, because no approved library does only zoom, Fit and focus.

## How other products do it

### Figma: pages and layers on the left, the canvas in the centre

[Mobbin screen](https://mobbin.com/screens/7b4311c9-85ea-41c3-ba09-984cfdc4df34?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A left panel shows the file name, then a "Pages" list with a plus, then a "Layers" list that names every frame on the page, such as "Sign In", "Checkout" and "Dashboard". The frames sit on the canvas with their names above them. A right panel has "Design" and "Prototype" tabs, the zoom percentage, and sections for the page, variables, styles and export. A tool bar floats at the bottom of the canvas.

Difference from LGTM Studio: the page list and the frame list are always visible as a panel. LGTM Studio folds both into two menus in the bar, so the canvas has the whole width.

### Framer: one component's variants laid out on a canvas

[Mobbin screen](https://mobbin.com/screens/88d60f1a-e622-460a-9d2c-465a1a1e22bd?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The left panel has three tabs, Pages, Layers and Assets. Assets shows a search field and a tree of components in folders. The canvas shows the selected component's four variants, each with a label above it such as "Primary · Hover". Breadcrumbs above the canvas name the project and the component. A small bar at the bottom holds the pointer tools and the zoom percentage, and a chat with an agent is in the right panel.

Difference from LGTM Studio: one component fills the canvas, and the tree of all components stays beside it. LGTM Studio shows a whole group of components on one sheet and reaches the others through the item menu.

### Replit: a design system as a page with its own side menu

[Mobbin screen](https://mobbin.com/screens/8459dad3-b886-444e-94dd-d26fad564348?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The design system opens in the preview pane beside the agent chat. Its side menu has a search field and labelled groups: Colors, Fonts, Layout, Actions, and Forms and inputs. The "Overview" entry shows the core palette as three swatches, the type styles, an "In use" sample form built from the tokens, and a row of button and badge variants. A "Save design system" button is at the top right.

Difference from LGTM Studio: the overview shows tokens and components together on one sheet, with a sample that uses them. LGTM Studio's Overview exists for Components only, and Palette has no overview.

### Claude: a design system as a list of folding rows

[Mobbin screen](https://mobbin.com/screens/f4221e3c-92ce-4e02-bf02-033ffe51aba4?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The chat is on the left, and the right side has "Design System" and "Design Files" tabs. The design system is a list of rows under five labels: Type, Colors, Spacing, Components and Brand. Each row has a chevron that opens it and a small dot at its right end. One row at the top shows a check in place of the dot.

Difference from LGTM Studio: every category is on one scrolling page as dense rows that open in place. LGTM Studio shows one item at a time on a canvas.

### MagicPath: design system tabs with counts and a light and dark switch

[Mobbin screen](https://mobbin.com/screens/f8e624cc-3d18-4981-ac3c-002fabb8f184?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A window titled "Manage design systems" lists the systems on the left, each with a strip of its main colours. The selected system has four tabs with counts: Colors 32, Typography 4, Spacing and Radius 1, and Shadows 6. A sun and moon switch at the right changes the mode. Each colour is a card with a swatch, a role name such as "Primary Foreground", its value, and one line on where it is used. "Delete", "Reset", "Export" and "Save" are in the header.

Difference from LGTM Studio: one mode shows at a time, and each token says where it is used. LGTM Studio shows the light and dark values in two columns of one row and says whether the code and the design file agree.

### v0: token fields beside a live preview of sample components

[Mobbin screen](https://mobbin.com/screens/c2f04e93-da86-461d-926b-026568f780ff?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A left column lists the colour tokens in pairs, such as "Primary" and "Primary Foreground", each as an editable field with a small swatch. The right side is a preview that draws real components with those tokens: a revenue card, a sign-up form, a chart and a payments table. A menu above the preview chooses which sample set to show, here "Cards". A sun and moon switch and a "Save" button are at the top right.

Difference from LGTM Studio: a token change is seen at once on whole components. LGTM Studio's Palette is a read-only table, and editing happens in OpenPencil.

### Sketch: a web view of a file's symbols, styles and colour variables

[Mobbin screen](https://mobbin.com/screens/15934aa1-c777-4300-99c1-c36946e2eed3?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

Breadcrumbs at the top lead to the file and end in a "Components" menu. A short side menu lists Symbols, Text Styles, Layer Styles and Color Variables, and two of them show a count. The colour variables are a grid of cards, each with a swatch, a name, a hex value and an opacity. Selecting a card opens an inspector on the right with the colour's value. An "Edit in" button at the top right opens the file in the desktop editor.

Difference from LGTM Studio: the web page is a viewer and the editor is a separate program, reached by one button. This is the same split as "Open in OpenPencil", with a side menu in place of LGTM Studio's two bar menus.

### Webflow: variables as a plain table of name and value

[Mobbin screen](https://mobbin.com/screens/67215f9e-74b8-49ca-9ddc-8ded3d65065c?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A "Variables" panel fills the window with a table of two columns, "Name" and "Value". Each row shows an icon for the variable's type, its name, a swatch and the hex value. A search field and a "New variable" button are at the top right, and a "New variable" row ends the list.

Difference from LGTM Studio: the table has no groups and one value for each variable. LGTM Studio groups its colour rows and adds the dark value, the CSS variable and the design file flag.

## Options for LGTM Studio

1. Replace the two bar menus with a narrow side list of categories and items that stays open. This follows Figma, Framer, Replit and Sketch. Every item is then one click away, and the canvas loses about 220 pixels of width on a laptop.
2. Show Palette and Components as one scrolling page of folding rows, and keep the canvas for Screens only. This follows Claude's design system list. It fits the owner's preference for dense rows that open on their own, and it gives up zoom for tokens and components, which do not need it.
3. Add an "Overview" item to Palette that shows the core colours, the type scale and one sample block built from them. This follows Replit. It costs one more sheet to keep current.
4. Let a component in the item list open alone on the canvas with all of its variants. This follows Framer. It needs one more level of address, such as `/design/components/actions/button`, and the group sheet stays as the level above.
5. Add a light and dark switch to the bar that redraws the whole sheet in one mode. This follows MagicPath and v0. The Colour table can keep both value columns, and the component and screen sheets gain a way to be checked in Dusk.
6. Add a "Used on" count or list to each token and each component. This follows MagicPath's usage line. The Mocks tab's Components view already holds a use count for a project, and the repository page would need the same count across projects.
7. Add a search field to the bar that finds a token, a component or a frame by name and focuses it on the canvas. This follows Webflow's variable search and Framer's asset search. It adds one control to a bar that is already full on a laptop.

## Ideas that are not in the feature list

- A search field that finds a token or a component by name ([Webflow](https://mobbin.com/screens/67215f9e-74b8-49ca-9ddc-8ded3d65065c?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A light and dark switch for the whole page ([MagicPath](https://mobbin.com/screens/f8e624cc-3d18-4981-ac3c-002fabb8f184?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- One line on each token that says where it is used ([MagicPath](https://mobbin.com/screens/f8e624cc-3d18-4981-ac3c-002fabb8f184?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- An "Export" button for the design system ([MagicPath](https://mobbin.com/screens/f8e624cc-3d18-4981-ac3c-002fabb8f184?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A live preview of sample components that redraws when a token changes ([v0](https://mobbin.com/screens/c2f04e93-da86-461d-926b-026568f780ff?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- An overview sheet with an "In use" sample built from the tokens ([Replit](https://mobbin.com/screens/8459dad3-b886-444e-94dd-d26fad564348?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A chat with an agent in a panel beside the design canvas ([Framer](https://mobbin.com/screens/88d60f1a-e622-460a-9d2c-465a1a1e22bd?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A menu that shows a colour's value in another format ([Sketch](https://mobbin.com/screens/15934aa1-c777-4300-99c1-c36946e2eed3?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).

## Searches that were run

- "Storybook component page with a sidebar of components and variant previews": 8 results, 4 usable. No result was from Storybook. The usable screens came from Framer, Replit, Claude and Sketch.
- "Zeroheight design system page with colour tokens": 8 results, 5 usable. No result was from Zeroheight. The usable screens came from MagicPath, Replit, v0 and Webflow, and a Subframe screen that is not cited.
- "Figma variables table with colour tokens in light and dark modes": 8 results, 4 usable. No result was from Figma. It repeated the Webflow, v0, MagicPath and Sketch screens.
- "Figma design file with a comment thread on a frame": run for the Mocks tab. One of its Figma screens is used here for the pages and layers panel.
- Storybook: not found on Mobbin.
- Zeroheight: not found on Mobbin.
- No reference was found for a table that compares code tokens with a design file, or for a grid of screen states by flow.
