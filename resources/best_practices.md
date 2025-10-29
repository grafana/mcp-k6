## k6 Scripting Best Practices

### Test Structure & Organization

1.  **Use Scenarios:** Structure your load tests with scenarios for clear and maintainable tests.
2.  **Use Thresholds:** Always define pass/fail criteria for your tests using thresholds.
3.  **Group Related Requests:** Use groups to organize and measure related requests together.
4.  **Externalize Configuration:** Use environment variables or JSON files for configuration that changes between environments.
5.  **Organize with Lifecycle Functions:** Use `setup()` and `teardown()` functions for test preparation and cleanup.
6.  **Structure Code with Modules:** Break complex tests into reusable modules and helper functions.

### Performance & Efficiency

7.  **Minimize Resource Usage:** Avoid unnecessary computations in the VU iteration function.
8.  **Use Batch Requests:** Leverage `http.batch()` for concurrent requests instead of sequential calls.
9.  **Optimize Data Generation:** Generate test data in `setup()` or use shared arrays rather than per-iteration generation.
10. **Use `SharedArray` for Large Datasets:** Import large datasets using `SharedArray` to avoid memory duplication across VUs.
11. **Prefer `sleep()` Over Fixed Delays:** Use `sleep()` with randomization to simulate realistic user behavior.

### Error Handling & Validation

12. **Implement Robust Error Handling:** Always check response status and handle errors gracefully.
13. **Use Checks for Assertions:** Prefer `check()` over throwing exceptions for non-critical validations.
14. **Validate Response Content:** Check response bodies, headers, and structure, not just status codes.
15. **Handle Network Failures:** Implement retry logic and timeout handling for unreliable networks.
16. **Use Custom Metrics:** Create custom metrics for business-specific measurements.

### Data Management

17. **Use Realistic Test Data:** Generate or import realistic datasets that reflect production scenarios.
18. **Implement Data Correlation:** Extract and reuse dynamic values between requests (e.g., authentication tokens).
19. **Manage Test Data Lifecycle:** Clean up test data created during tests to avoid pollution.
20. **Use Parameterization:** Make tests flexible with parameters for different environments and scenarios.

### Authentication & Security

21. **Secure Sensitive Data:** Never hardcode credentials; use environment variables or secure storage.
22. **Implement Proper Authentication:** Handle token refresh, session management, and multi-factor authentication.
23. **Test Security Headers:** Validate security-related response headers and SSL/TLS configuration.
24. **Use Realistic User Journeys:** Simulate actual user authentication flows, not just API calls.

### Monitoring & Observability

25. **Add Meaningful Tags:** Use tags to categorize requests and enable detailed analysis.
26. **Implement Custom Metrics:** Track business-specific KPIs beyond standard HTTP metrics.
27. **Use Trend Metrics:** Track response times, custom timers, and other time-based measurements.
28. **Add Request Naming:** Use descriptive names for requests to improve result readability.
29. **Log Important Events:** Use `console.log()` sparingly for debugging, avoid in production runs.

### Test Design Patterns

30. **Follow the AAA Pattern:** Structure tests with Arrange, Act, Assert phases.
31. **Use Page Object Models:** For browser testing, implement page objects for maintainability.
32. **Implement Think Time:** Add realistic delays between user actions using `sleep()`.
33. **Design for Scalability:** Structure tests to handle varying load levels and environments.
34. **Use Scenario Weights:** Balance different user behaviors using scenario weights.

### Code Quality & Maintainability

35. **Use Descriptive Naming:** Name functions, variables, and scenarios descriptively.
36. **Add Comments and Documentation:** Document complex logic and business rules.
37. **Follow JavaScript Best Practices:** Use modern JavaScript features and consistent coding style.
38. **Implement Constants:** Use constants for URLs, timeouts, and other configuration values.
39. **Version Control Integration:** Structure scripts for version control with proper gitignore patterns.

### Modern k6 Features & Protocols

54. **WebSocket Testing:** Use the `k6/ws` module for real-time communication testing, including chat applications, live updates, and streaming services.
55. **gRPC Testing:** Leverage gRPC protocol testing for microservices communication with protocol buffers (requires k6 with gRPC support).
56. **HTTP/2 Support:** Take advantage of automatic HTTP/2 protocol upgrades for more realistic modern web application testing.
57. **Native TypeScript Support:** Write k6 scripts directly in TypeScript with native support via esbuild integration - no compilation step required.
58. **Modern JavaScript Runtime:** Leverage k6's Grafana Sobek JavaScript runtime for ES6+ features and modern JavaScript syntax.
59. **Module System:** Use ES6 modules and k6's import system for better code organization and reusability.
60. **Custom Metrics:** Create business-specific metrics beyond standard HTTP metrics to track KPIs relevant to your application.

### Browser Testing & End-to-End Testing

40. **Choose the Right Testing Approach:** Use browser testing for UI interactions and frontend performance, HTTP testing for API performance and backend load testing.
41. **Leverage Browser Module for E2E Testing:** Use k6's browser module for comprehensive end-to-end website testing that includes JavaScript execution and DOM interaction.
42. **When to Use Browser Testing:**
    - Testing user journeys that involve complex frontend interactions
    - Validating client-side JavaScript functionality
    - Measuring real user experience metrics (Core Web Vitals)
    - Testing single-page applications (SPAs) with dynamic content
    - Verifying cross-browser compatibility
43. **When to Use HTTP Testing:**
    - Pure API performance testing
    - Backend load testing without frontend concerns
    - High-volume load testing (HTTP is more resource-efficient)
    - Testing microservices and REST/GraphQL APIs
    - CI/CD pipeline integration where browser overhead isn't needed
44. **Implement Page Object Pattern:** Structure browser tests with page objects for maintainability and reusability.
45. **Handle Asynchronous Operations:** Use proper waiting strategies (`waitForSelector`, `waitForLoadState`) for dynamic content and AJAX requests.
46. **Optimize Browser Performance:** Minimize browser overhead while maintaining realistic user simulation - consider headless mode for performance.
47. **Test Cross-Browser Compatibility:** Validate functionality across different browser engines when possible (currently supports Chromium-based browsers).
48. **Handle Pop-ups and Dialogs:** Implement proper handling for alerts, confirmations, and modal dialogs using browser event listeners.
49. **Validate Visual Elements:** Check for element visibility, text content, and proper rendering using locators and assertions.
50. **Simulate Real User Interactions:** Include realistic mouse movements, typing speed, and navigation patterns with appropriate `sleep()` calls.
51. **Manage Browser Context:** Properly handle browser instances, pages, and cleanup in multi-user scenarios to prevent memory leaks.
52. **Measure Frontend Performance:** Use browser testing to capture Core Web Vitals (LCP, FID, CLS) and other performance metrics.
53. **Test Progressive Web Apps:** Use browser testing for PWA-specific features like service workers, offline functionality, and app-like behaviors.

### Script Examples

#### Basic HTTP Test Structure
```javascript
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');

// Test configuration
export const options = {
  scenarios: {
    load_test: {
      executor: 'ramping-vus',
      startVUs: 1,
      stages: [
        { duration: '30s', target: 10 },
        { duration: '1m', target: 10 },
        { duration: '30s', target: 0 },
      ],
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<500'],
    errors: ['rate<0.1'],
  },
};

export default function() {
  const response = http.get('https://api.example.com/users');
  
  const success = check(response, {
    'status is 200': (r) => r.status === 200,
    'response time < 500ms': (r) => r.timings.duration < 500,
    'valid JSON response': (r) => {
      try {
        JSON.parse(r.body);
        return true;
      } catch {
        return false;
      }
    },
  });
  
  errorRate.add(!success);
  sleep(1);
}
```

#### Advanced Scenario with Authentication
```javascript
import http from 'k6/http';
import { check, sleep } from 'k6';
import { SharedArray } from 'k6/data';

// Load test data
const users = new SharedArray('users', function() {
  return JSON.parse(open('./users.json'));
});

export const options = {
  scenarios: {
    authenticated_users: {
      executor: 'per-vu-iterations',
      vus: 10,
      iterations: 5,
      maxDuration: '5m',
    },
  },
  thresholds: {
    'group_duration{group:::Login}': ['p(95)<2000'],
    'group_duration{group:::Browse Products}': ['p(95)<1000'],
    'http_req_failed': ['rate<0.01'],
  },
};

export function setup() {
  // Prepare test environment
  console.log('Setting up test environment...');
  return { timestamp: Date.now() };
}

export default function(data) {
  const user = users[__VU % users.length];
  let authToken;
  
  // Login flow
  group('Login', function() {
    const loginResponse = http.post('https://api.example.com/login', {
      username: user.username,
      password: user.password,
    });
    
    check(loginResponse, {
      'login successful': (r) => r.status === 200,
      'token received': (r) => r.json('token') !== undefined,
    });
    
    authToken = loginResponse.json('token');
  });
  
  // Authenticated requests
  if (authToken) {
    const headers = { Authorization: `Bearer ${authToken}` };
    
    group('Browse Products', function() {
      const response = http.get('https://api.example.com/products', { headers });
      
      check(response, {
        'products loaded': (r) => r.status === 200,
        'has products': (r) => r.json('products').length > 0,
      });
    });
  }
  
  sleep(Math.random() * 3 + 1); // Random think time 1-4 seconds
}

export function teardown(data) {
  // Clean up test environment
  console.log('Cleaning up test environment...');
}
```

#### Browser Testing Example
```javascript
import { browser } from 'k6/browser';
import { check, sleep } from 'k6';
import { SharedArray } from 'k6/data';

// Load test data
const users = new SharedArray('users', function() {
  return JSON.parse(open('./users.json'));
});

export const options = {
  scenarios: {
    browser_test: {
      executor: 'shared-iterations',
      options: {
        browser: {
          type: 'chromium',
          headless: true,
        },
      },
      vus: 3,
      iterations: 10,
      maxDuration: '10m',
    },
  },
  thresholds: {
    browser_web_vital_lcp: ['p(95)<3000'], // Largest Contentful Paint
    browser_web_vital_fid: ['p(95)<100'],  // First Input Delay
    browser_web_vital_cls: ['p(95)<0.1'],  // Cumulative Layout Shift
    checks: ['rate>0.9'],
  },
};

// Page Object Pattern
class LoginPage {
  constructor(page) {
    this.page = page;
    this.usernameInput = page.locator('input[name="username"]');
    this.passwordInput = page.locator('input[name="password"]');
    this.loginButton = page.locator('button[type="submit"]');
    this.errorMessage = page.locator('.error-message');
  }

  async login(username, password) {
    await this.usernameInput.fill(username);
    await this.passwordInput.fill(password);
    await this.loginButton.click();
  }

  async waitForError() {
    await this.errorMessage.waitFor();
    return this.errorMessage.textContent();
  }
}

class DashboardPage {
  constructor(page) {
    this.page = page;
    this.welcomeMessage = page.locator('.welcome-message');
    this.userMenu = page.locator('.user-menu');
    this.logoutButton = page.locator('button[data-testid="logout"]');
  }

  async waitForLoad() {
    await this.welcomeMessage.waitFor();
  }

  async logout() {
    await this.userMenu.click();
    await this.logoutButton.click();
  }

  async getWelcomeText() {
    return this.welcomeMessage.textContent();
  }
}

export default async function() {
  const context = browser.newContext();
  const page = context.newPage();
  
  try {
    const user = users[__VU % users.length];
    
    // Navigate to login page
    await page.goto('https://example.com/login');
    
    // Wait for page to load
    await page.waitForLoadState('networkidle');
    
    // Initialize page objects
    const loginPage = new LoginPage(page);
    const dashboardPage = new DashboardPage(page);
    
    // Perform login
    await loginPage.login(user.username, user.password);
    
    // Wait for navigation and check result
    try {
      await dashboardPage.waitForLoad();
      
      // Validate successful login
      const welcomeText = await dashboardPage.getWelcomeText();
      check(welcomeText, {
        'welcome message contains username': (text) => text.includes(user.username),
      });
      
      // Simulate user interaction
      await sleep(2); // Think time
      
      // Test navigation
      await page.click('a[href="/profile"]');
      await page.waitForLoadState('networkidle');
      
      check(page, {
        'profile page loaded': () => page.url().includes('/profile'),
        'page title correct': () => page.title().includes('Profile'),
      });
      
      // Test form interaction
      const profileForm = page.locator('form#profile-form');
      await profileForm.locator('input[name="displayName"]').fill('Test User');
      await profileForm.locator('button[type="submit"]').click();
      
      // Wait for success message
      const successMessage = page.locator('.success-message');
      await successMessage.waitFor();
      
      check(successMessage, {
        'profile updated successfully': (elem) => elem.isVisible(),
      });
      
      // Logout
      await dashboardPage.logout();
      
      // Verify logout
      await page.waitForURL('**/login');
      check(page, {
        'logout successful': () => page.url().includes('/login'),
      });
      
    } catch (error) {
      // Handle login failure
      try {
        const errorText = await loginPage.waitForError();
        check(errorText, {
          'login error displayed': (text) => text.length > 0,
        });
      } catch (e) {
        console.error('Login failed without error message:', error);
      }
    }
    
    // Test performance
    const performanceMetrics = page.evaluate(() => {
      const navigation = performance.getEntriesByType('navigation')[0];
      return {
        loadTime: navigation.loadEventEnd - navigation.loadEventStart,
        domContentLoaded: navigation.domContentLoadedEventEnd - navigation.domContentLoadedEventStart,
        firstPaint: performance.getEntriesByType('paint')[0]?.startTime || 0,
      };
    });
    
    check(performanceMetrics, {
      'page load time acceptable': (metrics) => metrics.loadTime < 3000,
      'DOM content loaded quickly': (metrics) => metrics.domContentLoaded < 2000,
    });
    
  } finally {
    // Always cleanup
    page.close();
    context.close();
  }
}
```

#### Browser Testing with Multiple User Scenarios
```javascript
import { browser } from 'k6/browser';
import { check, group, sleep } from 'k6';

export const options = {
  scenarios: {
    admin_user_flow: {
      executor: 'constant-vus',
      vus: 1,
      duration: '5m',
      options: {
        browser: {
          type: 'chromium',
          headless: true,
        },
      },
      tags: { userType: 'admin' },
    },
    regular_user_flow: {
      executor: 'constant-vus',
      vus: 2,
      duration: '5m',
      options: {
        browser: {
          type: 'chromium',
          headless: true,
        },
      },
      tags: { userType: 'regular' },
    },
  },
  thresholds: {
    'group_duration{group:::User Login}': ['p(95)<5000'],
    'group_duration{group:::Admin Operations}': ['p(95)<3000'],
    'browser_web_vital_lcp{userType:admin}': ['p(95)<2000'],
    'browser_web_vital_lcp{userType:regular}': ['p(95)<3000'],
  },
};

export default async function() {
  const context = browser.newContext({
    viewport: { width: 1920, height: 1080 },
  });
  const page = context.newPage();
  
  try {
    const userType = __ENV.K6_SCENARIO_TYPE || 'regular';
    
    group('User Login', function() {
      return performLogin(page, userType);
    });
    
    if (userType === 'admin') {
      group('Admin Operations', function() {
        return performAdminTasks(page);
      });
    } else {
      group('Regular User Operations', function() {
        return performRegularUserTasks(page);
      });
    }
    
  } finally {
    page.close();
    context.close();
  }
}

async function performLogin(page, userType) {
  await page.goto('https://example.com/login');
  
  const credentials = userType === 'admin' 
    ? { username: 'admin@example.com', password: 'admin123' }
    : { username: 'user@example.com', password: 'user123' };
  
  await page.fill('input[name="username"]', credentials.username);
  await page.fill('input[name="password"]', credentials.password);
  await page.click('button[type="submit"]');
  
  await page.waitForSelector('.dashboard', { timeout: 10000 });
  
  check(page, {
    'login successful': () => page.url().includes('/dashboard'),
    'user menu visible': () => page.locator('.user-menu').isVisible(),
  });
}

async function performAdminTasks(page) {
  // Navigate to admin panel
  await page.click('a[href="/admin"]');
  await page.waitForLoadState('networkidle');
  
  // Test admin-specific functionality
  await page.click('button[data-testid="create-user"]');
  await page.waitForSelector('.modal');
  
  // Fill user creation form
  await page.fill('input[name="newUserEmail"]', `test-${Date.now()}@example.com`);
  await page.fill('input[name="newUserName"]', 'Test User');
  await page.click('button[type="submit"]');
  
  // Wait for success notification
  await page.waitForSelector('.notification-success');
  
  check(page, {
    'user created successfully': () => page.locator('.notification-success').isVisible(),
  });
  
  sleep(1);
}

async function performRegularUserTasks(page) {
  // Browse products
  await page.click('a[href="/products"]');
  await page.waitForLoadState('networkidle');
  
  // Add item to cart
  await page.click('.product-card:first-child .add-to-cart');
  await page.waitForSelector('.cart-notification');
  
  check(page, {
    'item added to cart': () => page.locator('.cart-notification').isVisible(),
    'cart count updated': () => page.locator('.cart-count').textContent() > '0',
  });
  
  // Proceed to checkout
  await page.click('.cart-icon');
  await page.click('button[data-testid="checkout"]');
  await page.waitForSelector('.checkout-form');
  
  check(page, {
    'checkout page loaded': () => page.url().includes('/checkout'),
  });
  
  sleep(2);
}
```

### Quick Reference

**Common Patterns:**
- Use `scenarios` for test organization
- Implement `thresholds` for pass/fail criteria
- Use `check()` for assertions
- Add `sleep()` for realistic user behavior
- Use `group()` for request organization
- Implement custom metrics for business KPIs

**Performance Tips:**
- Use `SharedArray` for large datasets
- Batch requests with `http.batch()`
- Generate data in `setup()` when possible
- Avoid heavy computations in VU function
- Use appropriate think time and pacing

**Error Handling:**
- Always validate response status
- Check response content structure
- Handle authentication failures
- Implement retry logic for critical flows
- Use custom metrics to track errors

**Browser Testing:**
- Use browser module for end-to-end website testing
- Implement page object pattern for maintainability
- Use proper waiting strategies (`waitForSelector`, `waitForLoadState`)
- Handle asynchronous operations with `await`
- Always cleanup browser context and pages
- Test web vitals and performance metrics
- Simulate realistic user interactions and think time