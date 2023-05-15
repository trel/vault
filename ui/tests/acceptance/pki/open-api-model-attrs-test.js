/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import { currentURL } from '@ember/test-helpers';

import authPage from 'vault/tests/pages/auth';
import logout from 'vault/tests/pages/logout';
import { runCommands } from 'vault/tests/helpers/pki/pki-run-commands';

module('Acceptance | pki open api model attrs', function (hooks) {
  setupApplicationTest(hooks);

  hooks.beforeEach(async function () {
    this.roleModel = this.store.createRecord('pki/role');
    await logout.visit();
  });

  hooks.afterEach(async function () {
    await logout.visit();
    await authPage.login();
    // Cleanup engine
    await runCommands([`delete sys/mounts/${this.mountPath}`]);
    await logout.visit();
  });

  module('pki open api attrs', function (hooks) {
    setupMirage(hooks);

    test('it renders pki role models', async function (assert) {
      assert.strictEqual(currentURL(), `/vault/secrets/${this.mountPath}/pki/configuration/create`);
    });
  });
});
