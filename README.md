# 7Planner

A full-stack habit and routine tracking app built as a personal portfolio project. 7Planner lets users define recurring routines (daily, weekly, or custom intervals) and tracks completion over time, generating weekly performance reports and delivering daily push notifications.

---

## Screenshots

<div align="center">
  <table>
    <tr>
      <td align="center"><b>Welcome</b><br><img src="screenshots/08_welcome.png" width="200"/></td>
      <td align="center"><b>Login</b><br><img src="screenshots/07_login.png" width="200"/></td>
      <td align="center"><b>Today</b><br><img src="screenshots/01_today.png" width="200"/></td>
      <td align="center"><b>Dashboard</b><br><img src="screenshots/02_dashboard.png" width="200"/></td>
    </tr>
    <tr>
      <td align="center"><b>Routines</b><br><img src="screenshots/03_routines.png" width="200"/></td>
      <td align="center"><b>Routine Detail</b><br><img src="screenshots/05_routine_detail.png" width="200"/></td>
      <td align="center"><b>Create Routine</b><br><img src="screenshots/06_create_routine.png" width="200"/></td>
      <td align="center"><b>Profile</b><br><img src="screenshots/04_profile.png" width="200"/></td>
    </tr>
  </table>
</div>

---

## Features

- **Flexible recurrence** — routines repeat daily, on specific weekdays, or on custom intervals (every N days/weeks/months)
- **Daily task feed** — occurrences are generated automatically; complete, skip, or reschedule individual instances
- **Weekly performance report** — completion rate, streaks, on-time vs. late completions, skipped tasks
- **Push notifications** — daily morning reminder at 07:00 and weekly report notification via Firebase Cloud Messaging
- **Password reset via deep link** — forgot-password flow delivers a `sevenplanner://` deep link that opens the app directly to the reset screen
- **Category tagging** — work, health, personal, finance, learning, home, social, other
- **Timezone-aware** — user timezone stored at registration and applied to occurrence scheduling
- **JWT auth with refresh tokens** — 15-minute access token + long-lived refresh token stored in a server-side session table
- **Rate limiting** — login endpoint is rate-limited per email and per IP

---

## Tech Stack

### Backend — `server/`

| Layer | Technology |
|---|---|
| Language | Go 1.25 |
| Framework | Fiber v3 |
| Database | PostgreSQL |
| ORM | GORM (AutoMigrate) |
| Auth | JWT (golang-jwt v5), bcrypt |
| Push Notifications | Firebase Admin SDK (firebase.google.com/go v4) |
| Scheduler | robfig/cron v3 |
| Email | gomail v2 |
| Validation | go-playground/validator v10 |
| Config | godotenv |

### Mobile — `mobile/`

| Layer | Technology |
|---|---|
| Framework | Flutter 3.24 / Dart 3.5 |
| State Management | Riverpod 2.6 (StateNotifier) |
| Navigation | go_router 14.6 |
| HTTP Client | Dio 5.7 with interceptors & cookie jar |
| Secure Storage | flutter_secure_storage |
| Push Notifications | firebase_messaging + flutter_local_notifications |
| Deep Links | app_links 6.3 |
| Timezone Detection | flutter_timezone |

---

## Architecture

### Backend

```
server/
├── main.go                  # Fiber app setup, route registration, cron start
├── database/
│   ├── database.go          # GORM + PostgreSQL connection
│   └── functions.go         # Shared DB query helpers
├── middleware/
│   └── logger.go            # Structured logger (slog)
├── utils/
│   ├── validator.go         # Global validator instance
│   ├── password.go          # bcrypt helpers
│   └── mail.go              # SMTP email sender
└── modules/
    ├── auth/
    │   ├── models.go        # User, Session, PasswordResetToken, DTOs
    │   ├── handlers.go      # Register, Login, Refresh, Logout, ForgotPassword, ResetPassword, Me
    │   ├── jwt.go           # Token generation & validation
    │   └── middleware.go    # AccessTokenMiddleware, rate limiters
    ├── task/
    │   ├── models.go        # TaskTemplate, TaskOccurrence, DTOs
    │   └── handlers.go      # CRUD for templates, occurrence generation, dashboard, report
    └── notification/
        ├── firebase.go      # Firebase app init, SendPushToToken
        ├── functions.go     # Daily reminder & weekly report push logic, cron scheduler
        ├── handlers.go      # Register/delete device token endpoints
        └── models.go        # DeviceToken model
```

### Mobile

```
mobile/lib/
├── main.dart                          # Firebase init, ProviderScope, app entry
├── app.dart                           # GoRouter config, _AuthRouterNotifier, bottom nav shell
├── core/
│   ├── api/api_client.dart            # Dio singleton, token interceptor, auto-refresh
│   ├── models/user.dart               # User model
│   ├── providers/
│   │   ├── auth_provider.dart         # AuthState + AuthNotifier
│   │   └── onboarding_provider.dart
│   ├── services/notification_service.dart  # FCM token registration, local notifications
│   └── theme/app_theme.dart
└── features/
    ├── auth/           # Login, Register, ForgotPassword, ResetPassword
    ├── onboarding/     # Onboarding + Welcome screens
    ├── today/          # Today's task feed
    ├── dashboard/      # Stats overview
    ├── templates/      # Routines list, detail, create/edit
    ├── report/         # Weekly performance report
    └── profile/        # User info, timezone, logout
```

---

## API Reference

All routes are prefixed with `/v1`.

### Auth (public)

| Method | Path | Description |
|---|---|---|
| `POST` | `/auth/register` | Create account |
| `POST` | `/auth/login` | Login (rate-limited per email + IP) |
| `POST` | `/auth/refresh` | Rotate access token using refresh token |
| `POST` | `/auth/forgot-password` | Send password reset email |
| `POST` | `/auth/reset-password` | Reset password with token |
| `GET` | `/health` | Health check |

### Protected — `/u/*` (Bearer token required)

| Method | Path | Description |
|---|---|---|
| `GET` | `/me` | Current user info |
| `POST` | `/auth/logout` | Invalidate session |
| `GET` | `/tasks/today` | Today's occurrences + overdue |
| `PATCH` | `/tasks/occurrences/:id` | Update occurrence: `complete`, `undo`, `skip`, `reschedule` |
| `GET` | `/templates/` | List active routines |
| `POST` | `/templates/` | Create routine |
| `GET` | `/templates/:id` | Routine detail + occurrence history |
| `PATCH` | `/templates/:id` | Update routine fields |
| `PATCH` | `/templates/:id/status` | Soft-delete (`isActive=false`) |
| `GET` | `/dashboard` | Aggregate stats |
| `GET` | `/reports/weekly` | Weekly performance report |
| `POST` | `/notifications/tokens` | Register FCM device token |
| `DELETE` | `/notifications/tokens` | Deregister FCM device token |

---

## Data Model

### TaskTemplate

Represents a user-defined routine. Three recurrence modes:

| `repeatType` | Required fields | Example |
|---|---|---|
| `once` | `dueDate` | Single one-time task with a deadline |
| `weekly` | `weekDays` (`"1,3,5"`) | Every Monday, Wednesday, Friday |
| `interval` | `repeatUnit`, `repeatInterval`, `startDate` | Every 2 days, every 1 month |

### TaskOccurrence

Per-day instances of a template. A unique constraint on `(taskId, dueDate)` prevents duplicate generation. Statuses: `pending`, `completed`, `skipped`.

### Session

Server-side refresh token store — the token is stored as a SHA-256 hash, never in plain text. Supports multiple concurrent sessions (multi-device use).

---

## Running Locally

### Backend

Prerequisites: Go 1.21+, PostgreSQL, Firebase service account JSON

```bash
cd server
cp .env.example .env   # set DB_DSN, APP_PORT, FIREBASE_CREDENTIALS_PATH, SMTP_* vars

go run main.go
```

### Mobile

Prerequisites: Flutter 3.24+, Xcode (iOS) or Android Studio (Android)

```bash
cd mobile
flutter pub get
flutter run
```

The app connects to `http://127.0.0.1:5001/v1` by default. Change the base URL in `core/api/api_client.dart` for a deployed backend.

---

## Key Design Decisions

**Occurrence generation on-demand** — `TaskOccurrence` rows are generated lazily when the user opens the today feed or when the daily cron runs, rather than being pre-generated in bulk. This keeps the database lean and avoids a heavy background job.

**Soft delete for routines** — deactivating a routine sets `isActive=false` instead of deleting the row. Historical occurrences remain intact for the weekly report.

**Short-lived access tokens + server-side refresh sessions** — 15-minute JWTs limit the attack window; refresh tokens are hashed and stored in a `sessions` table so individual sessions can be revoked on logout without invalidating all devices.

**No local cache on mobile** — screens fetch fresh data on each load. Given the data volume (tens of occurrences per user per day), this avoids the complexity of a local DB or cache invalidation layer.

---

## Author

**Barış Nuri Korkmaz** — built as a personal portfolio project to practise full-stack development with Go and Flutter.

The backend was written entirely by hand. The Flutter frontend and test suites were developed with AI assistance (Claude and Antigravity).
