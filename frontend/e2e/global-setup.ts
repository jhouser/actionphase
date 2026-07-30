import { execSync } from 'child_process';
import path from 'path';
import { fileURLToPath } from 'url';

// Get __dirname equivalent in ES modules
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

/**
 * Global Setup for E2E Tests
 *
 * This script runs once before all E2E tests to ensure a clean, consistent database state.
 * It resets and reapplies all test fixtures.
 */
async function globalSetup() {
  // In the containerized stack the fixtures are loaded by `just load-e2e`
  // (which runs psql inside the backend container) *before* Playwright starts,
  // and the Playwright container has neither psql nor the fixture scripts.
  // E2E_SKIP_FIXTURE_SETUP=true tells us to skip the in-process loading.
  if (process.env.E2E_SKIP_FIXTURE_SETUP === 'true') {
    // eslint-disable-next-line no-console
    console.log('\n⏭️  Skipping in-process fixture setup (loaded externally via just load-e2e)\n');
    return;
  }

  // eslint-disable-next-line no-console
  console.log('\n🧹 Resetting E2E test fixtures...');

  try {
    const projectRoot = path.resolve(__dirname, '../..');

    // Apply common fixtures first
    // eslint-disable-next-line no-console
    console.log('📦 Applying common fixtures...');
    execSync('env DB_NAME=actionphase ./backend/pkg/db/test_fixtures/apply_common.sh', {
      stdio: 'inherit',
      cwd: projectRoot,
    });

    // Create E2E parallel worker users (workers 1-5; worker 0 already created by apply_common.sh)
    // eslint-disable-next-line no-console
    console.log('👥 Creating E2E parallel worker users...');
    execSync('env DB_NAME=actionphase ./backend/pkg/db/test_fixtures/apply_e2e_users.sh', {
      stdio: 'inherit',
      cwd: projectRoot,
    });

    // Apply worker-specific fixtures for all 6 workers (matches Playwright workers config)
    // eslint-disable-next-line no-console
    console.log('🔧 Applying worker-specific E2E fixtures for 6 parallel workers...');
    for (let workerIndex = 0; workerIndex <= 5; workerIndex++) {
      // eslint-disable-next-line no-console
      console.log(`  Worker ${workerIndex}...`);
      execSync(`env DB_NAME=actionphase ./backend/pkg/db/test_fixtures/apply_e2e_worker.sh ${workerIndex}`, {
        stdio: workerIndex === 0 ? 'inherit' : 'ignore', // Show output for Worker 0, hide for others
        cwd: projectRoot,
      });
    }

    // eslint-disable-next-line no-console
    console.log('✅ E2E test fixtures applied successfully for all workers!\n');
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('❌ Failed to apply E2E test fixtures:', error);
    throw error;
  }
}

export default globalSetup;
