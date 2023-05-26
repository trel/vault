### :hammer_and_wrench: Description

<!-- What code changed, and why? -->

### :flags: Feature Flag

<!-- Include any related feature flag names. -->

### :camera_flash: Screenshots

### :link: External Links

<!-- Issues, RFC, etc. Use the JIRA issue name (HCPE-123, HCP-123) to auto-link the PR to JIRA. -->

### :building_construction: How to Build and Test the Change

<!-- List steps to test your change on a local environment. -->

### :lock: Testing Auth-Related Changes

If your changes alter the pages for sign in (including the HCP landing page), the sign out flow, the Access Control pages, or SSO (creating, editing, or deleting SSO configurations), then some extra testing steps should be performed to ensure user authentication and authorization is not negatively impacted.

<details>
<summary><strong>Check E2E Tests</strong></summary>

Ensure e2e smoke tests are still passing before merging. Currently e2e tests do _not_ block merging. If you’re unsure why they’re failing for login or logout flows, reach out to the Accounts UI team and we can help debug!

</details>

<details>
<summary><strong>Perform Manual Testing Against Remote-Dev</strong></summary>

Do the following manual test cases pass?

- [ ] I can login with email/password, GitHub, and/or SSO\*
- [ ] I can login with the [breakglass link](https://support.hashicorp.com/hc/en-us/articles/4404718731923-SSO-sign-in-issues-Force-Login-with-Email-Password-)
- [ ] I can sign out
- [ ] I can sign up as a new user\*\*
  - [ ] I see the Terms of Service form
  - [ ] I must validate my email before being able to access the platform
- [ ] I can enable and disable MFA
- [ ] I can reset my password

\* If you do not belong to an SSO-enabled org and would like to test this functionality, please reach out to Accounts UI to be added to one

\*\* For this step, you can use your HashiCorp email `+` any string `@hashicorp.com` and it will still go to your HashiCorp email but will be a distinct user in our system. For example, I could create a user `john.doe+test@hashicorp.com` to create a new test user.

Use your best judgment as to which of these steps is the most applicable to the changes you’ve made. If you have any concerns, add the Accounts UI team as a PR reviewer and we will validate your changes.

</details>

### :+1: Definition of Done

- [ ] New functionality works?
- [ ] Tests added?
- [ ] Docs updated?

### :speech_balloon: Using the [Netlify feedback ladder](https://www.netlify.com/blog/2020/03/05/feedback-ladders-how-we-encode-code-reviews-at-netlify/)

- :mount_fuji: **[mountain]**: Blocking and requires immediate action.
- :moyai: **[boulder]**: Blocking.
- :white_circle: **[pebble]**: Non-blocking, but requires future action.
- :hourglass_flowing_sand: **[sand]**: Non-blocking, but requires future consideration.
- :sparkles: **[dust]**: Non-blocking. "Take it or leave it"
