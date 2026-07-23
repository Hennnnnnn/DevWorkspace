# PLAN Update - Remove Global User Activation

## Background

The original architecture of DevWorkspace required every newly
registered user to go through a global activation process before they
could use the application. During registration, each user was assigned a
unique SHA that had to be sent to an administrator for approval. Only
after the administrator activated the account could the user access the
application's features.

This workflow was designed around a centralized administration model,
where a global administrator controlled who was allowed to use the
platform.

## Why This Is Being Removed

The permission model has now changed from **global administration** to
**room-based ownership**.

Every authenticated user can immediately access DevWorkspace without
waiting for approval from an administrator. Instead of requiring a
global admin, ownership is determined by the room that a user creates.

Because of this architectural change, the SHA activation workflow no
longer provides meaningful value. It only adds unnecessary friction
during onboarding and makes the application harder to adopt.

## New Registration Flow

The new onboarding experience is intentionally simple:

``` text
Register
    ↓
Login
    ↓
Dashboard
    ↓
Create Room
    ↓
Become Room Owner
    ↓
Invite Members
```

Alternatively, users can join an existing room through an invitation:

``` text
Invitation
    ↓
Accept Invitation
    ↓
Become Member
```

## Removed Concepts

The following concepts are no longer part of the system architecture:

-   Global user activation
-   Pending user status
-   SHA submission to an administrator
-   Waiting for administrator approval
-   Global administrator approval before using the application

## Room-Based Ownership

Every authenticated user can:

-   Create unlimited rooms.
-   Automatically become the Owner of rooms they create.
-   Invite other users into their rooms.
-   Manage permissions only within rooms they own.
-   Join rooms created by other users as a Member.

A single user may simultaneously be:

-   **Owner** of Room A
-   **Member** of Room B
-   **Owner** of Room C

Permissions are scoped to each room instead of being applied globally
across the application.

## Benefits

This new architecture provides several advantages:

-   Faster onboarding with zero activation steps.
-   Lower friction for individual developers and small teams.
-   Better scalability through decentralized ownership.
-   Simpler permission management.
-   More intuitive collaboration model, similar to Discord, Slack,
    GitHub Organizations, and Notion Workspaces.
