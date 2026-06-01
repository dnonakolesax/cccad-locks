
    const schema = {
  "asyncapi": "3.1.0",
  "info": {
    "title": "cccAD Realtime Collaboration API",
    "version": "1.0.0",
    "description": "WebSocket protocol for cccAD collaborative 2D sketch editing.\n\nREST OpenAPI is used for initial document loading and regular HTTP commands.\nThis AsyncAPI document describes realtime WebSocket messages.\n\nRules:\n  - PostgreSQL sketch operation log is the persistent source of truth.\n  - op.committed is the only realtime event after which a client mutates real document state.\n  - presence.*, drag.preview, session ping/pong and locks are transient.\n  - Redis may be used for presence, locks and pub/sub fanout.\n  - S3 is used for previews, exports, imports and large snapshots, not live sketch state.\n"
  },
  "defaultContentType": "application/json",
  "servers": {
    "local": {
      "host": "localhost:8080",
      "protocol": "ws",
      "pathname": "/api/v1/sketches/{sketchId}/ws",
      "description": "Local development server.",
      "variables": {
        "sketchId": {
          "default": "sk_example",
          "description": "Sketch identifier."
        }
      }
    },
    "production": {
      "host": "api.cccad.example.com",
      "protocol": "wss",
      "pathname": "/api/v1/sketches/{sketchId}/ws",
      "description": "Production server.",
      "variables": {
        "sketchId": {
          "default": "sk_example",
          "description": "Sketch identifier."
        }
      }
    }
  },
  "channels": {
    "sketchRealtime": {
      "address": "/api/v1/sketches/{sketchId}/ws",
      "description": "Bidirectional WebSocket channel for one sketch document.\n\nClient should first load initial state through REST:\n  GET /api/v1/sketches/{sketchId}\n\nThen it should connect to this WebSocket and send session.join.\n",
      "parameters": {
        "sketchId": {
          "description": "Sketch identifier from the URL path."
        }
      },
      "messages": {
        "SessionJoin": {
          "name": "session.join",
          "title": "Join session",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "session.join"
                ],
                "x-parser-schema-id": "<anonymous-schema-2>"
              },
              "requestId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-3>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-4>"
              },
              "clientTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-5>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "lastSeenVersion",
                  "clientId",
                  "supportedProtocolVersion"
                ],
                "properties": {
                  "lastSeenVersion": {
                    "type": "integer",
                    "format": "int64",
                    "minimum": 0,
                    "x-parser-schema-id": "<anonymous-schema-6>"
                  },
                  "clientId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-7>"
                  },
                  "supportedProtocolVersion": {
                    "type": "integer",
                    "minimum": 1,
                    "x-parser-schema-id": "<anonymous-schema-8>"
                  }
                },
                "x-parser-schema-id": "SessionJoinPayload"
              }
            },
            "x-parser-schema-id": "SessionJoinMessage"
          },
          "examples": [
            {
              "name": "Join with last seen version",
              "payload": {
                "type": "session.join",
                "requestId": "req_join_1",
                "sketchId": "sk_123",
                "clientTime": "2026-05-21T10:00:00Z",
                "payload": {
                  "lastSeenVersion": 42,
                  "clientId": "browser_tab_01",
                  "supportedProtocolVersion": 1
                }
              }
            }
          ],
          "x-parser-unique-object-id": "SessionJoin"
        },
        "SessionJoined": {
          "name": "session.joined",
          "title": "Session joined",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "serverTime",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "session.joined"
                ],
                "x-parser-schema-id": "<anonymous-schema-9>"
              },
              "requestId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-10>"
              },
              "eventId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-11>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-12>"
              },
              "serverTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-13>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "protocolVersion",
                  "currentVersion",
                  "user",
                  "activeUsers",
                  "missingOpsAvailable"
                ],
                "properties": {
                  "protocolVersion": {
                    "type": "integer",
                    "x-parser-schema-id": "<anonymous-schema-14>"
                  },
                  "currentVersion": {
                    "type": "integer",
                    "format": "int64",
                    "x-parser-schema-id": "<anonymous-schema-15>"
                  },
                  "user": {
                    "type": "object",
                    "required": [
                      "userId",
                      "displayName",
                      "role",
                      "clientId"
                    ],
                    "properties": {
                      "userId": {
                        "type": "string",
                        "x-parser-schema-id": "<anonymous-schema-16>"
                      },
                      "displayName": {
                        "type": "string",
                        "x-parser-schema-id": "<anonymous-schema-17>"
                      },
                      "role": {
                        "type": "string",
                        "enum": [
                          "reader",
                          "editor",
                          "admin"
                        ],
                        "x-parser-schema-id": "SketchRole"
                      },
                      "clientId": {
                        "type": "string",
                        "x-parser-schema-id": "<anonymous-schema-18>"
                      }
                    },
                    "x-parser-schema-id": "RealtimeUser"
                  },
                  "activeUsers": {
                    "type": "array",
                    "items": "$ref:$.channels.sketchRealtime.messages.SessionJoined.payload.properties.payload.properties.user",
                    "x-parser-schema-id": "<anonymous-schema-19>"
                  },
                  "missingOpsAvailable": {
                    "type": "boolean",
                    "x-parser-schema-id": "<anonymous-schema-20>"
                  }
                },
                "x-parser-schema-id": "SessionJoinedPayload"
              }
            },
            "x-parser-schema-id": "SessionJoinedMessage"
          },
          "examples": [
            {
              "name": "Joined",
              "payload": {
                "type": "session.joined",
                "requestId": "req_join_1",
                "eventId": "evt_001",
                "sketchId": "sk_123",
                "serverTime": "2026-05-21T10:00:01Z",
                "payload": {
                  "protocolVersion": 1,
                  "currentVersion": 42,
                  "missingOpsAvailable": false,
                  "user": {
                    "userId": "usr_1",
                    "displayName": "David",
                    "role": "editor",
                    "clientId": "browser_tab_01"
                  },
                  "activeUsers": [
                    {
                      "userId": "usr_2",
                      "displayName": "Alex",
                      "role": "editor",
                      "clientId": "browser_tab_02"
                    }
                  ]
                }
              }
            }
          ],
          "x-parser-unique-object-id": "SessionJoined"
        },
        "SessionUserJoined": {
          "name": "session.user_joined",
          "title": "User joined",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "serverTime",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "session.user_joined"
                ],
                "x-parser-schema-id": "<anonymous-schema-21>"
              },
              "eventId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-22>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-23>"
              },
              "serverTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-24>"
              },
              "payload": "$ref:$.channels.sketchRealtime.messages.SessionJoined.payload.properties.payload.properties.user"
            },
            "x-parser-schema-id": "UserEventMessage"
          },
          "x-parser-unique-object-id": "SessionUserJoined"
        },
        "SessionUserLeft": {
          "name": "session.user_left",
          "title": "User left",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "serverTime",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "session.user_left"
                ],
                "x-parser-schema-id": "<anonymous-schema-25>"
              },
              "eventId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-26>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-27>"
              },
              "serverTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-28>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "userId",
                  "clientId",
                  "reason"
                ],
                "properties": {
                  "userId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-30>"
                  },
                  "clientId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-31>"
                  },
                  "reason": {
                    "type": "string",
                    "enum": [
                      "disconnect",
                      "access_revoked",
                      "server_shutdown",
                      "duplicate_connection",
                      "protocol_error"
                    ],
                    "x-parser-schema-id": "<anonymous-schema-32>"
                  }
                },
                "x-parser-schema-id": "<anonymous-schema-29>"
              }
            },
            "x-parser-schema-id": "UserLeftMessage"
          },
          "x-parser-unique-object-id": "SessionUserLeft"
        },
        "SessionPing": {
          "name": "session.ping",
          "title": "Ping",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "session.ping"
                ],
                "x-parser-schema-id": "<anonymous-schema-33>"
              },
              "requestId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-34>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-35>"
              },
              "clientTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-36>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "clientVersion"
                ],
                "properties": {
                  "clientVersion": {
                    "type": "integer",
                    "format": "int64",
                    "x-parser-schema-id": "<anonymous-schema-38>"
                  }
                },
                "x-parser-schema-id": "<anonymous-schema-37>"
              }
            },
            "x-parser-schema-id": "SessionPingMessage"
          },
          "x-parser-unique-object-id": "SessionPing"
        },
        "SessionPong": {
          "name": "session.pong",
          "title": "Pong",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "serverTime",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "session.pong"
                ],
                "x-parser-schema-id": "<anonymous-schema-39>"
              },
              "requestId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-40>"
              },
              "eventId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-41>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-42>"
              },
              "serverTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-43>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "currentVersion"
                ],
                "properties": {
                  "currentVersion": {
                    "type": "integer",
                    "format": "int64",
                    "x-parser-schema-id": "<anonymous-schema-45>"
                  }
                },
                "x-parser-schema-id": "<anonymous-schema-44>"
              }
            },
            "x-parser-schema-id": "SessionPongMessage"
          },
          "x-parser-unique-object-id": "SessionPong"
        },
        "SessionAccessRevoked": {
          "name": "session.access_revoked",
          "title": "Access revoked",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "serverTime",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "session.access_revoked"
                ],
                "x-parser-schema-id": "<anonymous-schema-46>"
              },
              "eventId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-47>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-48>"
              },
              "serverTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-49>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "message"
                ],
                "properties": {
                  "message": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-51>"
                  }
                },
                "x-parser-schema-id": "<anonymous-schema-50>"
              }
            },
            "x-parser-schema-id": "AccessRevokedMessage"
          },
          "x-parser-unique-object-id": "SessionAccessRevoked"
        },
        "PresenceCursor": {
          "name": "presence.cursor",
          "title": "Cursor presence",
          "summary": "Transient mouse cursor position broadcast so collaborators can render another user's cursor and display name in the sketch canvas.",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "presence.cursor"
                ],
                "x-parser-schema-id": "<anonymous-schema-52>"
              },
              "requestId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-53>"
              },
              "eventId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-54>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-55>"
              },
              "clientTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-56>"
              },
              "serverTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-57>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "actorUserId",
                  "clientId",
                  "cursorWorld"
                ],
                "properties": {
                  "actorUserId": {
                    "type": "string",
                    "description": "Authenticated user id that owns the moving cursor.",
                    "x-parser-schema-id": "<anonymous-schema-58>"
                  },
                  "actorUserName": {
                    "type": "string",
                    "description": "Display/login name to render near the cursor when available.",
                    "x-parser-schema-id": "<anonymous-schema-59>"
                  },
                  "userId": {
                    "type": "string",
                    "deprecated": true,
                    "description": "Legacy alias for actorUserId.",
                    "x-parser-schema-id": "<anonymous-schema-60>"
                  },
                  "userName": {
                    "type": "string",
                    "deprecated": true,
                    "description": "Legacy alias for actorUserName.",
                    "x-parser-schema-id": "<anonymous-schema-61>"
                  },
                  "clientId": {
                    "type": "string",
                    "description": "Browser tab/session id. Consumers use it to ignore their own cursor and de-duplicate multiple sessions.",
                    "x-parser-schema-id": "<anonymous-schema-62>"
                  },
                  "cursorWorld": {
                    "type": "object",
                    "required": [
                      "x",
                      "y"
                    ],
                    "properties": {
                      "x": {
                        "type": "number",
                        "format": "double"
                      },
                      "y": {
                        "type": "number",
                        "format": "double"
                      }
                    },
                    "x-parser-schema-id": "Vec2"
                  },
                  "x": {
                    "type": "number",
                    "format": "double",
                    "deprecated": true,
                    "description": "Legacy world X coordinate. Prefer cursorWorld.x.",
                    "x-parser-schema-id": "<anonymous-schema-63>"
                  },
                  "y": {
                    "type": "number",
                    "format": "double",
                    "deprecated": true,
                    "description": "Legacy world Y coordinate. Prefer cursorWorld.y.",
                    "x-parser-schema-id": "<anonymous-schema-64>"
                  },
                  "viewport": {
                    "type": "object",
                    "required": [
                      "scale",
                      "offsetX",
                      "offsetY"
                    ],
                    "properties": {
                      "scale": {
                        "type": "number",
                        "format": "double",
                        "exclusiveMinimum": 0,
                        "x-parser-schema-id": "<anonymous-schema-65>"
                      },
                      "offsetX": {
                        "type": "number",
                        "format": "double",
                        "x-parser-schema-id": "<anonymous-schema-66>"
                      },
                      "offsetY": {
                        "type": "number",
                        "format": "double",
                        "x-parser-schema-id": "<anonymous-schema-67>"
                      }
                    },
                    "x-parser-schema-id": "Viewport"
                  },
                  "ttlMs": {
                    "type": "integer",
                    "minimum": 100,
                    "default": 3000,
                    "description": "Client-side expiry for hiding stale cursors if no newer mouse move arrives.",
                    "x-parser-schema-id": "<anonymous-schema-68>"
                  }
                },
                "x-parser-schema-id": "PresenceCursorPayload"
              }
            },
            "x-parser-schema-id": "PresenceCursorMessage"
          },
          "x-parser-unique-object-id": "PresenceCursor"
        },
        "PresenceSelection": {
          "name": "presence.selection",
          "title": "Selection presence",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "presence.selection",
                  "presence.hover",
                  "presence.tool"
                ],
                "x-parser-schema-id": "<anonymous-schema-65>"
              },
              "requestId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-66>"
              },
              "eventId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-67>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-68>"
              },
              "clientTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-69>"
              },
              "serverTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-70>"
              },
              "payload": {
                "type": "object",
                "additionalProperties": true,
                "x-parser-schema-id": "<anonymous-schema-71>"
              }
            },
            "x-parser-schema-id": "GenericPresenceMessage"
          },
          "x-parser-unique-object-id": "PresenceSelection"
        },
        "PresenceHover": {
          "name": "presence.hover",
          "title": "Hover presence",
          "payload": "$ref:$.channels.sketchRealtime.messages.PresenceSelection.payload",
          "x-parser-unique-object-id": "PresenceHover"
        },
        "PresenceTool": {
          "name": "presence.tool",
          "title": "Tool presence",
          "payload": "$ref:$.channels.sketchRealtime.messages.PresenceSelection.payload",
          "x-parser-unique-object-id": "PresenceTool"
        },
        "DragBegin": {
          "name": "drag.begin",
          "title": "Begin drag",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "drag.begin"
                ],
                "x-parser-schema-id": "<anonymous-schema-72>"
              },
              "requestId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-73>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-74>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "entityId",
                  "kind",
                  "baseVersion"
                ],
                "properties": {
                  "entityId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-76>"
                  },
                  "kind": {
                    "type": "string",
                    "enum": [
                      "point",
                      "line",
                      "circle",
                      "arc",
                      "entity"
                    ],
                    "x-parser-schema-id": "<anonymous-schema-77>"
                  },
                  "baseVersion": {
                    "type": "integer",
                    "format": "int64",
                    "x-parser-schema-id": "<anonymous-schema-78>"
                  }
                },
                "x-parser-schema-id": "<anonymous-schema-75>"
              }
            },
            "x-parser-schema-id": "DragBeginMessage"
          },
          "x-parser-unique-object-id": "DragBegin"
        },
        "DragBeginAccepted": {
          "name": "drag.begin.accepted",
          "title": "Drag begin accepted",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "serverTime",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "drag.begin.accepted"
                ],
                "x-parser-schema-id": "<anonymous-schema-79>"
              },
              "requestId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-80>"
              },
              "eventId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-81>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-82>"
              },
              "serverTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-83>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "lockId",
                  "lockedEntityIds"
                ],
                "properties": {
                  "lockId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-85>"
                  },
                  "componentId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-86>"
                  },
                  "lockedEntityIds": {
                    "type": "array",
                    "items": {
                      "type": "string",
                      "x-parser-schema-id": "<anonymous-schema-88>"
                    },
                    "x-parser-schema-id": "<anonymous-schema-87>"
                  }
                },
                "x-parser-schema-id": "<anonymous-schema-84>"
              }
            },
            "x-parser-schema-id": "DragBeginAcceptedMessage"
          },
          "x-parser-unique-object-id": "DragBeginAccepted"
        },
        "DragBeginRejected": {
          "name": "drag.begin.rejected",
          "title": "Drag begin rejected",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "serverTime",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "drag.begin.rejected"
                ],
                "x-parser-schema-id": "<anonymous-schema-89>"
              },
              "requestId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-90>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-91>"
              },
              "serverTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-92>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "reason"
                ],
                "properties": {
                  "reason": {
                    "type": "string",
                    "enum": [
                      "lock_conflict",
                      "permission_denied",
                      "invalid_reference",
                      "stale_base_version"
                    ],
                    "x-parser-schema-id": "<anonymous-schema-94>"
                  },
                  "lockedByUserId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-95>"
                  },
                  "lockId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-96>"
                  }
                },
                "x-parser-schema-id": "<anonymous-schema-93>"
              }
            },
            "x-parser-schema-id": "DragBeginRejectedMessage"
          },
          "x-parser-unique-object-id": "DragBeginRejected"
        },
        "DragPreview": {
          "name": "drag.preview",
          "title": "Drag preview",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "drag.preview"
                ],
                "x-parser-schema-id": "<anonymous-schema-97>"
              },
              "requestId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-98>"
              },
              "eventId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-99>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-100>"
              },
              "clientTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-101>"
              },
              "serverTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-102>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "entityId",
                  "target"
                ],
                "properties": {
                  "userId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-104>"
                  },
                  "clientId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-105>"
                  },
                  "lockId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-106>"
                  },
                  "entityId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-107>"
                  },
                  "target": {
                    "type": "object",
                    "required": [
                      "x",
                      "y"
                    ],
                    "properties": {
                      "x": {
                        "type": "number",
                        "format": "double",
                        "x-parser-schema-id": "<anonymous-schema-108>"
                      },
                      "y": {
                        "type": "number",
                        "format": "double",
                        "x-parser-schema-id": "<anonymous-schema-109>"
                      }
                    },
                    "x-parser-schema-id": "Vec2"
                  }
                },
                "x-parser-schema-id": "<anonymous-schema-103>"
              }
            },
            "x-parser-schema-id": "DragPreviewMessage"
          },
          "x-parser-unique-object-id": "DragPreview"
        },
        "DragCommit": {
          "name": "drag.commit",
          "title": "Commit drag",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "drag.commit"
                ],
                "x-parser-schema-id": "<anonymous-schema-110>"
              },
              "requestId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-111>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-112>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "baseVersion",
                  "clientOpId",
                  "op"
                ],
                "properties": {
                  "lockId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-114>"
                  },
                  "baseVersion": {
                    "type": "integer",
                    "format": "int64",
                    "x-parser-schema-id": "<anonymous-schema-115>"
                  },
                  "clientOpId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-116>"
                  },
                  "op": {
                    "type": "object",
                    "required": [
                      "type"
                    ],
                    "description": "Persistent sketch operation. Concrete shape is selected by the type field. Discriminator is intentionally avoided for better AsyncAPI tooling compatibility.",
                    "properties": {
                      "type": {
                        "type": "string",
                        "enum": [
                          "create_point",
                          "create_line",
                          "create_circle",
                          "create_arc",
                          "create_polyline",
                          "create_rectangle",
                          "move_point",
                          "delete_entity",
                          "add_constraint",
                          "remove_constraint",
                          "add_dimension",
                          "set_dimension_value",
                          "remove_dimension"
                        ],
                        "x-parser-schema-id": "<anonymous-schema-117>"
                      },
                      "x": {
                        "type": "number",
                        "format": "double",
                        "x-parser-schema-id": "<anonymous-schema-118>"
                      },
                      "y": {
                        "type": "number",
                        "format": "double",
                        "x-parser-schema-id": "<anonymous-schema-119>"
                      },
                      "pointId": {
                        "type": "string",
                        "x-parser-schema-id": "<anonymous-schema-120>"
                      },
                      "entityId": {
                        "type": "string",
                        "x-parser-schema-id": "<anonymous-schema-121>"
                      },
                      "constraintId": {
                        "type": "string",
                        "x-parser-schema-id": "<anonymous-schema-122>"
                      },
                      "dimensionId": {
                        "type": "string",
                        "x-parser-schema-id": "<anonymous-schema-123>"
                      },
                      "value": {
                        "type": "number",
                        "format": "double",
                        "x-parser-schema-id": "<anonymous-schema-124>"
                      },
                      "radius": {
                        "type": "number",
                        "format": "double",
                        "exclusiveMinimum": 0,
                        "x-parser-schema-id": "<anonymous-schema-125>"
                      },
                      "clockwise": {
                        "type": "boolean",
                        "x-parser-schema-id": "<anonymous-schema-126>"
                      },
                      "closed": {
                        "type": "boolean",
                        "x-parser-schema-id": "<anonymous-schema-127>"
                      },
                      "start": {
                        "type": "object",
                        "required": [
                          "kind"
                        ],
                        "properties": {
                          "kind": {
                            "type": "string",
                            "enum": [
                              "existing_point",
                              "new_point"
                            ],
                            "x-parser-schema-id": "<anonymous-schema-128>"
                          },
                          "pointId": {
                            "type": "string",
                            "x-parser-schema-id": "<anonymous-schema-129>"
                          },
                          "x": {
                            "type": "number",
                            "format": "double",
                            "x-parser-schema-id": "<anonymous-schema-130>"
                          },
                          "y": {
                            "type": "number",
                            "format": "double",
                            "x-parser-schema-id": "<anonymous-schema-131>"
                          }
                        },
                        "x-parser-schema-id": "PointRefOrNew"
                      },
                      "end": "$ref:$.channels.sketchRealtime.messages.DragCommit.payload.properties.payload.properties.op.properties.start",
                      "center": "$ref:$.channels.sketchRealtime.messages.DragCommit.payload.properties.payload.properties.op.properties.start",
                      "target": "$ref:$.channels.sketchRealtime.messages.DragPreview.payload.properties.payload.properties.target",
                      "cornerA": "$ref:$.channels.sketchRealtime.messages.DragPreview.payload.properties.payload.properties.target",
                      "cornerB": "$ref:$.channels.sketchRealtime.messages.DragPreview.payload.properties.payload.properties.target",
                      "points": {
                        "type": "array",
                        "items": "$ref:$.channels.sketchRealtime.messages.DragPreview.payload.properties.payload.properties.target",
                        "x-parser-schema-id": "<anonymous-schema-132>"
                      },
                      "constraint": {
                        "type": "object",
                        "required": [
                          "type",
                          "refs"
                        ],
                        "properties": {
                          "id": {
                            "type": "string",
                            "x-parser-schema-id": "<anonymous-schema-133>"
                          },
                          "type": {
                            "type": "string",
                            "enum": [
                              "coincident",
                              "horizontal",
                              "vertical",
                              "parallel",
                              "perpendicular",
                              "tangent",
                              "equal",
                              "fixed",
                              "midpoint",
                              "concentric"
                            ],
                            "x-parser-schema-id": "<anonymous-schema-134>"
                          },
                          "refs": {
                            "type": "array",
                            "minItems": 1,
                            "items": {
                              "type": "string",
                              "x-parser-schema-id": "<anonymous-schema-136>"
                            },
                            "x-parser-schema-id": "<anonymous-schema-135>"
                          },
                          "branch": {
                            "type": "string",
                            "enum": [
                              "external",
                              "internal"
                            ],
                            "x-parser-schema-id": "<anonymous-schema-137>"
                          },
                          "status": {
                            "type": "string",
                            "enum": [
                              "active",
                              "invalid",
                              "removed"
                            ],
                            "x-parser-schema-id": "<anonymous-schema-138>"
                          }
                        },
                        "x-parser-schema-id": "Constraint"
                      },
                      "dimension": {
                        "type": "object",
                        "required": [
                          "type",
                          "refs",
                          "value",
                          "driving"
                        ],
                        "properties": {
                          "id": {
                            "type": "string",
                            "x-parser-schema-id": "<anonymous-schema-139>"
                          },
                          "type": {
                            "type": "string",
                            "enum": [
                              "distance",
                              "radius",
                              "diameter",
                              "angle"
                            ],
                            "x-parser-schema-id": "<anonymous-schema-140>"
                          },
                          "refs": {
                            "type": "array",
                            "minItems": 1,
                            "items": {
                              "type": "string",
                              "x-parser-schema-id": "<anonymous-schema-142>"
                            },
                            "x-parser-schema-id": "<anonymous-schema-141>"
                          },
                          "value": {
                            "type": "number",
                            "format": "double",
                            "x-parser-schema-id": "<anonymous-schema-143>"
                          },
                          "driving": {
                            "type": "boolean",
                            "x-parser-schema-id": "<anonymous-schema-144>"
                          },
                          "orientation": {
                            "type": "string",
                            "enum": [
                              "cw",
                              "ccw"
                            ],
                            "x-parser-schema-id": "<anonymous-schema-145>"
                          }
                        },
                        "x-parser-schema-id": "Dimension"
                      }
                    },
                    "additionalProperties": true,
                    "x-parser-schema-id": "SketchOperation"
                  }
                },
                "x-parser-schema-id": "<anonymous-schema-113>"
              }
            },
            "x-parser-schema-id": "DragCommitMessage"
          },
          "x-parser-unique-object-id": "DragCommit"
        },
        "DragCancel": {
          "name": "drag.cancel",
          "title": "Cancel drag",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "drag.cancel"
                ],
                "x-parser-schema-id": "<anonymous-schema-146>"
              },
              "requestId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-147>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-148>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "entityId"
                ],
                "properties": {
                  "lockId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-150>"
                  },
                  "entityId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-151>"
                  }
                },
                "x-parser-schema-id": "<anonymous-schema-149>"
              }
            },
            "x-parser-schema-id": "DragCancelMessage"
          },
          "x-parser-unique-object-id": "DragCancel"
        },
        "DragCancelled": {
          "name": "drag.cancelled",
          "title": "Drag cancelled",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "serverTime",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "drag.cancelled"
                ],
                "x-parser-schema-id": "<anonymous-schema-152>"
              },
              "eventId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-153>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-154>"
              },
              "serverTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-155>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "userId",
                  "entityId"
                ],
                "properties": {
                  "userId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-157>"
                  },
                  "entityId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-158>"
                  },
                  "lockId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-159>"
                  }
                },
                "x-parser-schema-id": "<anonymous-schema-156>"
              }
            },
            "x-parser-schema-id": "DragCancelledMessage"
          },
          "x-parser-unique-object-id": "DragCancelled"
        },
        "OpSubmit": {
          "name": "op.submit",
          "title": "Submit persistent operation",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "op.submit"
                ],
                "x-parser-schema-id": "<anonymous-schema-160>"
              },
              "requestId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-161>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-162>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "baseVersion",
                  "clientOpId",
                  "op"
                ],
                "properties": {
                  "baseVersion": {
                    "type": "integer",
                    "format": "int64",
                    "x-parser-schema-id": "<anonymous-schema-163>"
                  },
                  "clientOpId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-164>"
                  },
                  "op": "$ref:$.channels.sketchRealtime.messages.DragCommit.payload.properties.payload.properties.op"
                },
                "x-parser-schema-id": "OpSubmitPayload"
              }
            },
            "x-parser-schema-id": "OpSubmitMessage"
          },
          "x-parser-unique-object-id": "OpSubmit"
        },
        "OpCommitted": {
          "name": "op.committed",
          "title": "Operation committed",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "serverTime",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "op.committed"
                ],
                "x-parser-schema-id": "<anonymous-schema-165>"
              },
              "requestId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-166>"
              },
              "eventId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-167>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-168>"
              },
              "serverTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-169>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "opId",
                  "version",
                  "actorUserId",
                  "op",
                  "patch",
                  "solveStatus",
                  "affectedEntityIds"
                ],
                "properties": {
                  "opId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-170>"
                  },
                  "version": {
                    "type": "integer",
                    "format": "int64",
                    "x-parser-schema-id": "<anonymous-schema-171>"
                  },
                  "actorUserId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-172>"
                  },
                  "clientOpId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-173>"
                  },
                  "op": "$ref:$.channels.sketchRealtime.messages.DragCommit.payload.properties.payload.properties.op",
                  "patch": {
                    "type": "object",
                    "properties": {
                      "entities": {
                        "type": "object",
                        "additionalProperties": true,
                        "x-parser-schema-id": "<anonymous-schema-174>"
                      },
                      "constraints": {
                        "type": "object",
                        "additionalProperties": true,
                        "x-parser-schema-id": "<anonymous-schema-175>"
                      },
                      "dimensions": {
                        "type": "object",
                        "additionalProperties": true,
                        "x-parser-schema-id": "<anonymous-schema-176>"
                      },
                      "deletedEntityIds": {
                        "type": "array",
                        "items": {
                          "type": "string",
                          "x-parser-schema-id": "<anonymous-schema-178>"
                        },
                        "x-parser-schema-id": "<anonymous-schema-177>"
                      },
                      "deletedConstraintIds": {
                        "type": "array",
                        "items": {
                          "type": "string",
                          "x-parser-schema-id": "<anonymous-schema-180>"
                        },
                        "x-parser-schema-id": "<anonymous-schema-179>"
                      },
                      "deletedDimensionIds": {
                        "type": "array",
                        "items": {
                          "type": "string",
                          "x-parser-schema-id": "<anonymous-schema-182>"
                        },
                        "x-parser-schema-id": "<anonymous-schema-181>"
                      }
                    },
                    "x-parser-schema-id": "SketchPatch"
                  },
                  "solveStatus": {
                    "type": "object",
                    "required": [
                      "status",
                      "degreesOfFreedom"
                    ],
                    "properties": {
                      "status": {
                        "type": "string",
                        "enum": [
                          "ok",
                          "under_constrained",
                          "fully_constrained",
                          "over_constrained",
                          "inconsistent",
                          "numerical_failure"
                        ],
                        "x-parser-schema-id": "<anonymous-schema-183>"
                      },
                      "degreesOfFreedom": {
                        "type": "integer",
                        "x-parser-schema-id": "<anonymous-schema-184>"
                      },
                      "diagnostics": {
                        "type": "array",
                        "items": {
                          "type": "object",
                          "required": [
                            "level",
                            "code",
                            "message"
                          ],
                          "properties": {
                            "level": {
                              "type": "string",
                              "enum": [
                                "info",
                                "warning",
                                "error"
                              ],
                              "x-parser-schema-id": "<anonymous-schema-186>"
                            },
                            "code": {
                              "type": "string",
                              "x-parser-schema-id": "<anonymous-schema-187>"
                            },
                            "message": {
                              "type": "string",
                              "x-parser-schema-id": "<anonymous-schema-188>"
                            },
                            "entityIds": {
                              "type": "array",
                              "items": {
                                "type": "string",
                                "x-parser-schema-id": "<anonymous-schema-190>"
                              },
                              "x-parser-schema-id": "<anonymous-schema-189>"
                            },
                            "constraintIds": {
                              "type": "array",
                              "items": {
                                "type": "string",
                                "x-parser-schema-id": "<anonymous-schema-192>"
                              },
                              "x-parser-schema-id": "<anonymous-schema-191>"
                            },
                            "dimensionIds": {
                              "type": "array",
                              "items": {
                                "type": "string",
                                "x-parser-schema-id": "<anonymous-schema-194>"
                              },
                              "x-parser-schema-id": "<anonymous-schema-193>"
                            }
                          },
                          "x-parser-schema-id": "SolverDiagnostic"
                        },
                        "x-parser-schema-id": "<anonymous-schema-185>"
                      }
                    },
                    "x-parser-schema-id": "SolveStatus"
                  },
                  "affectedEntityIds": {
                    "type": "array",
                    "items": {
                      "type": "string",
                      "x-parser-schema-id": "<anonymous-schema-196>"
                    },
                    "x-parser-schema-id": "<anonymous-schema-195>"
                  }
                },
                "x-parser-schema-id": "OpCommittedPayload"
              }
            },
            "x-parser-schema-id": "OpCommittedMessage"
          },
          "x-parser-unique-object-id": "OpCommitted"
        },
        "OpRejected": {
          "name": "op.rejected",
          "title": "Operation rejected",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "serverTime",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "op.rejected"
                ],
                "x-parser-schema-id": "<anonymous-schema-197>"
              },
              "requestId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-198>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-199>"
              },
              "serverTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-200>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "clientOpId",
                  "currentVersion",
                  "reason",
                  "message"
                ],
                "properties": {
                  "clientOpId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-201>"
                  },
                  "currentVersion": {
                    "type": "integer",
                    "format": "int64",
                    "x-parser-schema-id": "<anonymous-schema-202>"
                  },
                  "reason": {
                    "type": "string",
                    "enum": [
                      "stale_base_version",
                      "permission_denied",
                      "lock_conflict",
                      "invalid_reference",
                      "invalid_operation",
                      "solver_inconsistent",
                      "solver_over_constrained",
                      "solver_numerical_failure",
                      "unsupported_operation"
                    ],
                    "x-parser-schema-id": "<anonymous-schema-203>"
                  },
                  "message": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-204>"
                  },
                  "diagnostics": {
                    "type": "array",
                    "items": "$ref:$.channels.sketchRealtime.messages.OpCommitted.payload.properties.payload.properties.solveStatus.properties.diagnostics.items",
                    "x-parser-schema-id": "<anonymous-schema-205>"
                  }
                },
                "x-parser-schema-id": "OpRejectedPayload"
              }
            },
            "x-parser-schema-id": "OpRejectedMessage"
          },
          "x-parser-unique-object-id": "OpRejected"
        },
        "OpsBatch": {
          "name": "ops.batch",
          "title": "Operations batch",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "serverTime",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "ops.batch"
                ],
                "x-parser-schema-id": "<anonymous-schema-206>"
              },
              "eventId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-207>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-208>"
              },
              "serverTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-209>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "fromVersion",
                  "toVersion",
                  "ops"
                ],
                "properties": {
                  "fromVersion": {
                    "type": "integer",
                    "format": "int64",
                    "x-parser-schema-id": "<anonymous-schema-211>"
                  },
                  "toVersion": {
                    "type": "integer",
                    "format": "int64",
                    "x-parser-schema-id": "<anonymous-schema-212>"
                  },
                  "ops": {
                    "type": "array",
                    "items": {
                      "type": "object",
                      "required": [
                        "opId",
                        "version",
                        "actorUserId",
                        "op"
                      ],
                      "properties": {
                        "opId": {
                          "type": "string",
                          "x-parser-schema-id": "<anonymous-schema-214>"
                        },
                        "version": {
                          "type": "integer",
                          "format": "int64",
                          "x-parser-schema-id": "<anonymous-schema-215>"
                        },
                        "actorUserId": {
                          "type": "string",
                          "x-parser-schema-id": "<anonymous-schema-216>"
                        },
                        "clientOpId": {
                          "type": "string",
                          "x-parser-schema-id": "<anonymous-schema-217>"
                        },
                        "createdAt": {
                          "type": "string",
                          "format": "date-time",
                          "x-parser-schema-id": "<anonymous-schema-218>"
                        },
                        "op": "$ref:$.channels.sketchRealtime.messages.DragCommit.payload.properties.payload.properties.op"
                      },
                      "x-parser-schema-id": "CommittedOperation"
                    },
                    "x-parser-schema-id": "<anonymous-schema-213>"
                  }
                },
                "x-parser-schema-id": "<anonymous-schema-210>"
              }
            },
            "x-parser-schema-id": "OpsBatchMessage"
          },
          "x-parser-unique-object-id": "OpsBatch"
        },
        "LockAcquire": {
          "name": "lock.acquire",
          "title": "Acquire lock",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "lock.acquire"
                ],
                "x-parser-schema-id": "<anonymous-schema-219>"
              },
              "requestId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-220>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-221>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "scope",
                  "mode",
                  "ttlMs"
                ],
                "properties": {
                  "scope": {
                    "type": "object",
                    "required": [
                      "type"
                    ],
                    "properties": {
                      "type": {
                        "type": "string",
                        "enum": [
                          "entity",
                          "constraint",
                          "dimension",
                          "constraint_component"
                        ],
                        "x-parser-schema-id": "<anonymous-schema-222>"
                      },
                      "entityId": {
                        "type": "string",
                        "x-parser-schema-id": "<anonymous-schema-223>"
                      },
                      "constraintId": {
                        "type": "string",
                        "x-parser-schema-id": "<anonymous-schema-224>"
                      },
                      "dimensionId": {
                        "type": "string",
                        "x-parser-schema-id": "<anonymous-schema-225>"
                      },
                      "componentId": {
                        "type": "string",
                        "x-parser-schema-id": "<anonymous-schema-226>"
                      }
                    },
                    "x-parser-schema-id": "LockScope"
                  },
                  "mode": {
                    "type": "string",
                    "enum": [
                      "edit"
                    ],
                    "x-parser-schema-id": "<anonymous-schema-227>"
                  },
                  "ttlMs": {
                    "type": "integer",
                    "minimum": 1000,
                    "x-parser-schema-id": "<anonymous-schema-228>"
                  }
                },
                "x-parser-schema-id": "LockAcquirePayload"
              }
            },
            "x-parser-schema-id": "LockAcquireMessage"
          },
          "x-parser-unique-object-id": "LockAcquire"
        },
        "LockAcquired": {
          "name": "lock.acquired",
          "title": "Lock acquired",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "serverTime",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "lock.acquired"
                ],
                "x-parser-schema-id": "<anonymous-schema-229>"
              },
              "requestId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-230>"
              },
              "eventId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-231>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-232>"
              },
              "serverTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-233>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "lockId",
                  "userId",
                  "scope",
                  "entityIds",
                  "expiresAt"
                ],
                "properties": {
                  "lockId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-235>"
                  },
                  "userId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-236>"
                  },
                  "scope": "$ref:$.channels.sketchRealtime.messages.LockAcquire.payload.properties.payload.properties.scope",
                  "entityIds": {
                    "type": "array",
                    "items": {
                      "type": "string",
                      "x-parser-schema-id": "<anonymous-schema-238>"
                    },
                    "x-parser-schema-id": "<anonymous-schema-237>"
                  },
                  "expiresAt": {
                    "type": "string",
                    "format": "date-time",
                    "x-parser-schema-id": "<anonymous-schema-239>"
                  }
                },
                "x-parser-schema-id": "<anonymous-schema-234>"
              }
            },
            "x-parser-schema-id": "LockAcquiredMessage"
          },
          "x-parser-unique-object-id": "LockAcquired"
        },
        "LockRejected": {
          "name": "lock.rejected",
          "title": "Lock rejected",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "serverTime",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "lock.rejected"
                ],
                "x-parser-schema-id": "<anonymous-schema-240>"
              },
              "requestId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-241>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-242>"
              },
              "serverTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-243>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "reason"
                ],
                "properties": {
                  "reason": {
                    "type": "string",
                    "enum": [
                      "already_locked",
                      "permission_denied",
                      "invalid_scope"
                    ],
                    "x-parser-schema-id": "<anonymous-schema-245>"
                  },
                  "lockedByUserId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-246>"
                  },
                  "lockId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-247>"
                  }
                },
                "x-parser-schema-id": "<anonymous-schema-244>"
              }
            },
            "x-parser-schema-id": "LockRejectedMessage"
          },
          "x-parser-unique-object-id": "LockRejected"
        },
        "LockRefresh": {
          "name": "lock.refresh",
          "title": "Refresh lock",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "lock.refresh"
                ],
                "x-parser-schema-id": "<anonymous-schema-248>"
              },
              "requestId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-249>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-250>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "lockId",
                  "ttlMs"
                ],
                "properties": {
                  "lockId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-252>"
                  },
                  "ttlMs": {
                    "type": "integer",
                    "minimum": 1000,
                    "x-parser-schema-id": "<anonymous-schema-253>"
                  }
                },
                "x-parser-schema-id": "<anonymous-schema-251>"
              }
            },
            "x-parser-schema-id": "LockRefreshMessage"
          },
          "x-parser-unique-object-id": "LockRefresh"
        },
        "LockRefreshed": {
          "name": "lock.refreshed",
          "title": "Lock refreshed",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "serverTime",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "lock.refreshed"
                ],
                "x-parser-schema-id": "<anonymous-schema-254>"
              },
              "requestId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-255>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-256>"
              },
              "serverTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-257>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "lockId",
                  "expiresAt"
                ],
                "properties": {
                  "lockId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-259>"
                  },
                  "expiresAt": {
                    "type": "string",
                    "format": "date-time",
                    "x-parser-schema-id": "<anonymous-schema-260>"
                  }
                },
                "x-parser-schema-id": "<anonymous-schema-258>"
              }
            },
            "x-parser-schema-id": "LockRefreshedMessage"
          },
          "x-parser-unique-object-id": "LockRefreshed"
        },
        "LockRelease": {
          "name": "lock.release",
          "title": "Release lock",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "lock.release"
                ],
                "x-parser-schema-id": "<anonymous-schema-261>"
              },
              "requestId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-262>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-263>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "lockId"
                ],
                "properties": {
                  "lockId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-265>"
                  }
                },
                "x-parser-schema-id": "<anonymous-schema-264>"
              }
            },
            "x-parser-schema-id": "LockReleaseMessage"
          },
          "x-parser-unique-object-id": "LockRelease"
        },
        "LockReleased": {
          "name": "lock.released",
          "title": "Lock released",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "serverTime",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "lock.released"
                ],
                "x-parser-schema-id": "<anonymous-schema-266>"
              },
              "requestId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-267>"
              },
              "eventId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-268>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-269>"
              },
              "serverTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-270>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "lockId",
                  "reason"
                ],
                "properties": {
                  "lockId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-272>"
                  },
                  "reason": {
                    "type": "string",
                    "enum": [
                      "released",
                      "expired",
                      "user_disconnected",
                      "operation_committed",
                      "server_revoked"
                    ],
                    "x-parser-schema-id": "<anonymous-schema-273>"
                  },
                  "userId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-274>"
                  }
                },
                "x-parser-schema-id": "<anonymous-schema-271>"
              }
            },
            "x-parser-schema-id": "LockReleasedMessage"
          },
          "x-parser-unique-object-id": "LockReleased"
        },
        "StateResyncRequired": {
          "name": "state.resync_required",
          "title": "State resync required",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "serverTime",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "state.resync_required"
                ],
                "x-parser-schema-id": "<anonymous-schema-275>"
              },
              "eventId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-276>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-277>"
              },
              "serverTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-278>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "currentVersion",
                  "reason",
                  "recommendedAction"
                ],
                "properties": {
                  "currentVersion": {
                    "type": "integer",
                    "format": "int64",
                    "x-parser-schema-id": "<anonymous-schema-280>"
                  },
                  "reason": {
                    "type": "string",
                    "enum": [
                      "client_too_far_behind",
                      "missed_events",
                      "server_restart",
                      "protocol_error"
                    ],
                    "x-parser-schema-id": "<anonymous-schema-281>"
                  },
                  "recommendedAction": {
                    "type": "string",
                    "enum": [
                      "fetch_snapshot",
                      "fetch_ops",
                      "reconnect"
                    ],
                    "x-parser-schema-id": "<anonymous-schema-282>"
                  }
                },
                "x-parser-schema-id": "<anonymous-schema-279>"
              }
            },
            "x-parser-schema-id": "StateResyncRequiredMessage"
          },
          "x-parser-unique-object-id": "StateResyncRequired"
        },
        "StateSnapshot": {
          "name": "state.snapshot",
          "title": "State snapshot",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "serverTime",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "state.snapshot"
                ],
                "x-parser-schema-id": "<anonymous-schema-283>"
              },
              "eventId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-284>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-285>"
              },
              "serverTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-286>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "version",
                  "document"
                ],
                "properties": {
                  "version": {
                    "type": "integer",
                    "format": "int64",
                    "x-parser-schema-id": "<anonymous-schema-288>"
                  },
                  "document": {
                    "type": "object",
                    "required": [
                      "plane"
                    ],
                    "properties": {
                      "plane": {
                        "type": "object",
                        "required": [
                          "origin",
                          "normal",
                          "xAxis"
                        ],
                        "properties": {
                          "origin": {
                            "type": "object",
                            "required": [
                              "x",
                              "y",
                              "z"
                            ],
                            "properties": {
                              "x": {
                                "type": "number",
                                "format": "double"
                              },
                              "y": {
                                "type": "number",
                                "format": "double"
                              },
                              "z": {
                                "type": "number",
                                "format": "double"
                              }
                            }
                          },
                          "normal": {
                            "type": "object",
                            "required": [
                              "x",
                              "y",
                              "z"
                            ],
                            "properties": {
                              "x": {
                                "type": "number",
                                "format": "double"
                              },
                              "y": {
                                "type": "number",
                                "format": "double"
                              },
                              "z": {
                                "type": "number",
                                "format": "double"
                              }
                            }
                          },
                          "xAxis": {
                            "type": "object",
                            "required": [
                              "x",
                              "y",
                              "z"
                            ],
                            "properties": {
                              "x": {
                                "type": "number",
                                "format": "double"
                              },
                              "y": {
                                "type": "number",
                                "format": "double"
                              },
                              "z": {
                                "type": "number",
                                "format": "double"
                              }
                            }
                          }
                        },
                        "description": "Sketch plane in model coordinates. normal and xAxis must be non-zero vectors."
                      },
                      "entities": {
                        "type": "object",
                        "additionalProperties": true,
                        "x-parser-schema-id": "<anonymous-schema-289>"
                      },
                      "constraints": {
                        "type": "object",
                        "additionalProperties": true,
                        "x-parser-schema-id": "<anonymous-schema-290>"
                      },
                      "dimensions": {
                        "type": "object",
                        "additionalProperties": true,
                        "x-parser-schema-id": "<anonymous-schema-291>"
                      },
                      "groups": {
                        "type": "object",
                        "additionalProperties": true,
                        "x-parser-schema-id": "<anonymous-schema-292>"
                      },
                      "materializedGeometry": {
                        "type": "object",
                        "additionalProperties": true,
                        "x-parser-schema-id": "<anonymous-schema-293>"
                      },
                      "solveStatus": "$ref:$.channels.sketchRealtime.messages.OpCommitted.payload.properties.payload.properties.solveStatus"
                    },
                    "x-parser-schema-id": "SketchDocument"
                  }
                },
                "x-parser-schema-id": "<anonymous-schema-287>"
              }
            },
            "x-parser-schema-id": "StateSnapshotMessage"
          },
          "x-parser-unique-object-id": "StateSnapshot"
        },
        "StatePatch": {
          "name": "state.patch",
          "title": "State patch",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "serverTime",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "state.patch"
                ],
                "x-parser-schema-id": "<anonymous-schema-294>"
              },
              "eventId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-295>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-296>"
              },
              "serverTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-297>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "version",
                  "patch"
                ],
                "properties": {
                  "version": {
                    "type": "integer",
                    "format": "int64",
                    "x-parser-schema-id": "<anonymous-schema-299>"
                  },
                  "patch": "$ref:$.channels.sketchRealtime.messages.OpCommitted.payload.properties.payload.properties.patch"
                },
                "x-parser-schema-id": "<anonymous-schema-298>"
              }
            },
            "x-parser-schema-id": "StatePatchMessage"
          },
          "x-parser-unique-object-id": "StatePatch"
        },
        "PermissionUpdated": {
          "name": "permission.updated",
          "title": "Permission updated",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "serverTime",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "permission.updated"
                ],
                "x-parser-schema-id": "<anonymous-schema-300>"
              },
              "eventId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-301>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-302>"
              },
              "serverTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-303>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "targetUserId",
                  "role",
                  "changedByUserId"
                ],
                "properties": {
                  "targetUserId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-305>"
                  },
                  "role": "$ref:$.channels.sketchRealtime.messages.SessionJoined.payload.properties.payload.properties.user.properties.role",
                  "changedByUserId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-306>"
                  }
                },
                "x-parser-schema-id": "<anonymous-schema-304>"
              }
            },
            "x-parser-schema-id": "PermissionUpdatedMessage"
          },
          "x-parser-unique-object-id": "PermissionUpdated"
        },
        "PermissionRevoked": {
          "name": "permission.revoked",
          "title": "Permission revoked",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "serverTime",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "permission.revoked"
                ],
                "x-parser-schema-id": "<anonymous-schema-307>"
              },
              "eventId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-308>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-309>"
              },
              "serverTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-310>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "targetUserId",
                  "changedByUserId"
                ],
                "properties": {
                  "targetUserId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-312>"
                  },
                  "changedByUserId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-313>"
                  }
                },
                "x-parser-schema-id": "<anonymous-schema-311>"
              }
            },
            "x-parser-schema-id": "PermissionRevokedMessage"
          },
          "x-parser-unique-object-id": "PermissionRevoked"
        },
        "ConflictCreated": {
          "name": "conflict.created",
          "title": "Conflict created",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "serverTime",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "conflict.created"
                ],
                "x-parser-schema-id": "<anonymous-schema-314>"
              },
              "eventId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-315>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-316>"
              },
              "serverTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-317>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "conflictId",
                  "conflictType",
                  "status",
                  "affectedEntityIds",
                  "causedByOps",
                  "message"
                ],
                "properties": {
                  "conflictId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-319>"
                  },
                  "conflictType": {
                    "type": "string",
                    "enum": [
                      "over_constrained",
                      "inconsistent_constraints",
                      "dangling_reference",
                      "dimension_conflict",
                      "solver_branch_conflict"
                    ],
                    "x-parser-schema-id": "<anonymous-schema-320>"
                  },
                  "status": {
                    "type": "string",
                    "enum": [
                      "open",
                      "resolved",
                      "ignored"
                    ],
                    "x-parser-schema-id": "<anonymous-schema-321>"
                  },
                  "affectedEntityIds": {
                    "type": "array",
                    "items": {
                      "type": "string",
                      "x-parser-schema-id": "<anonymous-schema-323>"
                    },
                    "x-parser-schema-id": "<anonymous-schema-322>"
                  },
                  "causedByOps": {
                    "type": "array",
                    "items": {
                      "type": "string",
                      "x-parser-schema-id": "<anonymous-schema-325>"
                    },
                    "x-parser-schema-id": "<anonymous-schema-324>"
                  },
                  "message": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-326>"
                  },
                  "possibleResolutions": {
                    "type": "array",
                    "items": {
                      "type": "object",
                      "additionalProperties": true,
                      "x-parser-schema-id": "<anonymous-schema-328>"
                    },
                    "x-parser-schema-id": "<anonymous-schema-327>"
                  }
                },
                "x-parser-schema-id": "<anonymous-schema-318>"
              }
            },
            "x-parser-schema-id": "ConflictCreatedMessage"
          },
          "x-parser-unique-object-id": "ConflictCreated"
        },
        "ConflictResolved": {
          "name": "conflict.resolved",
          "title": "Conflict resolved",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "serverTime",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "conflict.resolved"
                ],
                "x-parser-schema-id": "<anonymous-schema-329>"
              },
              "eventId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-330>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-331>"
              },
              "serverTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-332>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "conflictId",
                  "resolvedByUserId"
                ],
                "properties": {
                  "conflictId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-334>"
                  },
                  "resolvedByUserId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-335>"
                  },
                  "resolutionOpId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-336>"
                  }
                },
                "x-parser-schema-id": "<anonymous-schema-333>"
              }
            },
            "x-parser-schema-id": "ConflictResolvedMessage"
          },
          "x-parser-unique-object-id": "ConflictResolved"
        },
        "Error": {
          "name": "error",
          "title": "Error",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "sketchId",
              "serverTime",
              "payload"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "error"
                ],
                "x-parser-schema-id": "<anonymous-schema-337>"
              },
              "requestId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-338>"
              },
              "eventId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-339>"
              },
              "sketchId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-340>"
              },
              "serverTime": {
                "type": "string",
                "format": "date-time",
                "x-parser-schema-id": "<anonymous-schema-341>"
              },
              "payload": {
                "type": "object",
                "required": [
                  "code",
                  "message"
                ],
                "properties": {
                  "code": {
                    "type": "string",
                    "enum": [
                      "INVALID_MESSAGE",
                      "INVALID_PAYLOAD",
                      "AUTH_REQUIRED",
                      "PERMISSION_DENIED",
                      "SKETCH_NOT_FOUND",
                      "STALE_VERSION",
                      "LOCK_CONFLICT",
                      "RATE_LIMITED",
                      "INTERNAL_ERROR"
                    ],
                    "x-parser-schema-id": "<anonymous-schema-342>"
                  },
                  "message": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-343>"
                  },
                  "details": {
                    "type": "object",
                    "additionalProperties": true,
                    "x-parser-schema-id": "<anonymous-schema-344>"
                  }
                },
                "x-parser-schema-id": "ErrorPayload"
              }
            },
            "x-parser-schema-id": "ErrorMessage"
          },
          "x-parser-unique-object-id": "Error"
        }
      },
      "x-parser-unique-object-id": "sketchRealtime"
    }
  },
  "operations": {
    "joinSession": {
      "action": "send",
      "channel": "$ref:$.channels.sketchRealtime",
      "title": "Join realtime sketch session",
      "messages": [
        "$ref:$.channels.sketchRealtime.messages.SessionJoin"
      ],
      "reply": {
        "channel": "$ref:$.channels.sketchRealtime",
        "messages": [
          "$ref:$.channels.sketchRealtime.messages.SessionJoined",
          "$ref:$.channels.sketchRealtime.messages.Error"
        ]
      },
      "x-parser-unique-object-id": "joinSession"
    },
    "heartbeat": {
      "action": "send",
      "channel": "$ref:$.channels.sketchRealtime",
      "title": "Heartbeat ping",
      "messages": [
        "$ref:$.channels.sketchRealtime.messages.SessionPing"
      ],
      "reply": {
        "channel": "$ref:$.channels.sketchRealtime",
        "messages": [
          "$ref:$.channels.sketchRealtime.messages.SessionPong",
          "$ref:$.channels.sketchRealtime.messages.Error"
        ]
      },
      "x-parser-unique-object-id": "heartbeat"
    },
    "sendPresence": {
      "action": "send",
      "channel": "$ref:$.channels.sketchRealtime",
      "title": "Send transient presence events",
      "messages": [
        "$ref:$.channels.sketchRealtime.messages.PresenceCursor",
        "$ref:$.channels.sketchRealtime.messages.PresenceSelection",
        "$ref:$.channels.sketchRealtime.messages.PresenceHover",
        "$ref:$.channels.sketchRealtime.messages.PresenceTool"
      ],
      "x-parser-unique-object-id": "sendPresence"
    },
    "beginDrag": {
      "action": "send",
      "channel": "$ref:$.channels.sketchRealtime",
      "title": "Begin drag lifecycle",
      "messages": [
        "$ref:$.channels.sketchRealtime.messages.DragBegin"
      ],
      "reply": {
        "channel": "$ref:$.channels.sketchRealtime",
        "messages": [
          "$ref:$.channels.sketchRealtime.messages.DragBeginAccepted",
          "$ref:$.channels.sketchRealtime.messages.DragBeginRejected",
          "$ref:$.channels.sketchRealtime.messages.Error"
        ]
      },
      "x-parser-unique-object-id": "beginDrag"
    },
    "sendDragPreview": {
      "action": "send",
      "channel": "$ref:$.channels.sketchRealtime",
      "title": "Send transient drag preview",
      "messages": [
        "$ref:$.channels.sketchRealtime.messages.DragPreview"
      ],
      "x-parser-unique-object-id": "sendDragPreview"
    },
    "commitDrag": {
      "action": "send",
      "channel": "$ref:$.channels.sketchRealtime",
      "title": "Commit drag as persistent operation",
      "messages": [
        "$ref:$.channels.sketchRealtime.messages.DragCommit"
      ],
      "reply": {
        "channel": "$ref:$.channels.sketchRealtime",
        "messages": [
          "$ref:$.channels.sketchRealtime.messages.OpCommitted",
          "$ref:$.channels.sketchRealtime.messages.OpRejected",
          "$ref:$.channels.sketchRealtime.messages.Error"
        ]
      },
      "x-parser-unique-object-id": "commitDrag"
    },
    "submitOperation": {
      "action": "send",
      "channel": "$ref:$.channels.sketchRealtime",
      "title": "Submit persistent sketch operation",
      "messages": [
        "$ref:$.channels.sketchRealtime.messages.OpSubmit"
      ],
      "reply": {
        "channel": "$ref:$.channels.sketchRealtime",
        "messages": [
          "$ref:$.channels.sketchRealtime.messages.OpCommitted",
          "$ref:$.channels.sketchRealtime.messages.OpRejected",
          "$ref:$.channels.sketchRealtime.messages.Error"
        ]
      },
      "x-parser-unique-object-id": "submitOperation"
    },
    "acquireLock": {
      "action": "send",
      "channel": "$ref:$.channels.sketchRealtime",
      "title": "Acquire soft edit lock",
      "messages": [
        "$ref:$.channels.sketchRealtime.messages.LockAcquire"
      ],
      "reply": {
        "channel": "$ref:$.channels.sketchRealtime",
        "messages": [
          "$ref:$.channels.sketchRealtime.messages.LockAcquired",
          "$ref:$.channels.sketchRealtime.messages.LockRejected",
          "$ref:$.channels.sketchRealtime.messages.Error"
        ]
      },
      "x-parser-unique-object-id": "acquireLock"
    },
    "refreshLock": {
      "action": "send",
      "channel": "$ref:$.channels.sketchRealtime",
      "title": "Refresh soft edit lock",
      "messages": [
        "$ref:$.channels.sketchRealtime.messages.LockRefresh"
      ],
      "reply": {
        "channel": "$ref:$.channels.sketchRealtime",
        "messages": [
          "$ref:$.channels.sketchRealtime.messages.LockRefreshed",
          "$ref:$.channels.sketchRealtime.messages.Error"
        ]
      },
      "x-parser-unique-object-id": "refreshLock"
    },
    "releaseLock": {
      "action": "send",
      "channel": "$ref:$.channels.sketchRealtime",
      "title": "Release soft edit lock",
      "messages": [
        "$ref:$.channels.sketchRealtime.messages.LockRelease"
      ],
      "reply": {
        "channel": "$ref:$.channels.sketchRealtime",
        "messages": [
          "$ref:$.channels.sketchRealtime.messages.LockReleased",
          "$ref:$.channels.sketchRealtime.messages.Error"
        ]
      },
      "x-parser-unique-object-id": "releaseLock"
    }
  },
  "components": {
    "messages": {
      "SessionJoin": "$ref:$.channels.sketchRealtime.messages.SessionJoin",
      "SessionJoined": "$ref:$.channels.sketchRealtime.messages.SessionJoined",
      "SessionUserJoined": "$ref:$.channels.sketchRealtime.messages.SessionUserJoined",
      "SessionUserLeft": "$ref:$.channels.sketchRealtime.messages.SessionUserLeft",
      "SessionPing": "$ref:$.channels.sketchRealtime.messages.SessionPing",
      "SessionPong": "$ref:$.channels.sketchRealtime.messages.SessionPong",
      "SessionAccessRevoked": "$ref:$.channels.sketchRealtime.messages.SessionAccessRevoked",
      "PresenceCursor": "$ref:$.channels.sketchRealtime.messages.PresenceCursor",
      "PresenceSelection": "$ref:$.channels.sketchRealtime.messages.PresenceSelection",
      "PresenceHover": "$ref:$.channels.sketchRealtime.messages.PresenceHover",
      "PresenceTool": "$ref:$.channels.sketchRealtime.messages.PresenceTool",
      "DragBegin": "$ref:$.channels.sketchRealtime.messages.DragBegin",
      "DragBeginAccepted": "$ref:$.channels.sketchRealtime.messages.DragBeginAccepted",
      "DragBeginRejected": "$ref:$.channels.sketchRealtime.messages.DragBeginRejected",
      "DragPreview": "$ref:$.channels.sketchRealtime.messages.DragPreview",
      "DragCommit": "$ref:$.channels.sketchRealtime.messages.DragCommit",
      "DragCancel": "$ref:$.channels.sketchRealtime.messages.DragCancel",
      "DragCancelled": "$ref:$.channels.sketchRealtime.messages.DragCancelled",
      "OpSubmit": "$ref:$.channels.sketchRealtime.messages.OpSubmit",
      "OpCommitted": "$ref:$.channels.sketchRealtime.messages.OpCommitted",
      "OpRejected": "$ref:$.channels.sketchRealtime.messages.OpRejected",
      "OpsBatch": "$ref:$.channels.sketchRealtime.messages.OpsBatch",
      "LockAcquire": "$ref:$.channels.sketchRealtime.messages.LockAcquire",
      "LockAcquired": "$ref:$.channels.sketchRealtime.messages.LockAcquired",
      "LockRejected": "$ref:$.channels.sketchRealtime.messages.LockRejected",
      "LockRefresh": "$ref:$.channels.sketchRealtime.messages.LockRefresh",
      "LockRefreshed": "$ref:$.channels.sketchRealtime.messages.LockRefreshed",
      "LockRelease": "$ref:$.channels.sketchRealtime.messages.LockRelease",
      "LockReleased": "$ref:$.channels.sketchRealtime.messages.LockReleased",
      "StateResyncRequired": "$ref:$.channels.sketchRealtime.messages.StateResyncRequired",
      "StateSnapshot": "$ref:$.channels.sketchRealtime.messages.StateSnapshot",
      "StatePatch": "$ref:$.channels.sketchRealtime.messages.StatePatch",
      "PermissionUpdated": "$ref:$.channels.sketchRealtime.messages.PermissionUpdated",
      "PermissionRevoked": "$ref:$.channels.sketchRealtime.messages.PermissionRevoked",
      "ConflictCreated": "$ref:$.channels.sketchRealtime.messages.ConflictCreated",
      "ConflictResolved": "$ref:$.channels.sketchRealtime.messages.ConflictResolved",
      "Error": "$ref:$.channels.sketchRealtime.messages.Error"
    },
    "schemas": {
      "RealtimeEnvelope": {
        "type": "object",
        "required": [
          "type",
          "sketchId",
          "payload"
        ],
        "properties": {
          "type": {
            "type": "string",
            "x-parser-schema-id": "<anonymous-schema-345>"
          },
          "requestId": {
            "type": "string",
            "x-parser-schema-id": "<anonymous-schema-346>"
          },
          "eventId": {
            "type": "string",
            "x-parser-schema-id": "<anonymous-schema-347>"
          },
          "sketchId": {
            "type": "string",
            "x-parser-schema-id": "<anonymous-schema-348>"
          },
          "clientTime": {
            "type": "string",
            "format": "date-time",
            "x-parser-schema-id": "<anonymous-schema-349>"
          },
          "serverTime": {
            "type": "string",
            "format": "date-time",
            "x-parser-schema-id": "<anonymous-schema-350>"
          },
          "payload": {
            "type": "object",
            "additionalProperties": true,
            "x-parser-schema-id": "<anonymous-schema-351>"
          }
        },
        "x-parser-schema-id": "RealtimeEnvelope"
      },
      "SessionJoinMessage": "$ref:$.channels.sketchRealtime.messages.SessionJoin.payload",
      "SessionJoinPayload": "$ref:$.channels.sketchRealtime.messages.SessionJoin.payload.properties.payload",
      "SessionJoinedMessage": "$ref:$.channels.sketchRealtime.messages.SessionJoined.payload",
      "SessionJoinedPayload": "$ref:$.channels.sketchRealtime.messages.SessionJoined.payload.properties.payload",
      "RealtimeUser": "$ref:$.channels.sketchRealtime.messages.SessionJoined.payload.properties.payload.properties.user",
      "SketchRole": "$ref:$.channels.sketchRealtime.messages.SessionJoined.payload.properties.payload.properties.user.properties.role",
      "UserEventMessage": "$ref:$.channels.sketchRealtime.messages.SessionUserJoined.payload",
      "UserLeftMessage": "$ref:$.channels.sketchRealtime.messages.SessionUserLeft.payload",
      "SessionPingMessage": "$ref:$.channels.sketchRealtime.messages.SessionPing.payload",
      "SessionPongMessage": "$ref:$.channels.sketchRealtime.messages.SessionPong.payload",
      "AccessRevokedMessage": "$ref:$.channels.sketchRealtime.messages.SessionAccessRevoked.payload",
      "PresenceCursorMessage": "$ref:$.channels.sketchRealtime.messages.PresenceCursor.payload",
      "PresenceCursorPayload": "$ref:$.channels.sketchRealtime.messages.PresenceCursor.payload.properties.payload",
      "Viewport": "$ref:$.channels.sketchRealtime.messages.PresenceCursor.payload.properties.payload.properties.viewport",
      "PresenceSelectionMessage": "$ref:$.channels.sketchRealtime.messages.PresenceSelection.payload",
      "PresenceHoverMessage": "$ref:$.channels.sketchRealtime.messages.PresenceSelection.payload",
      "PresenceToolMessage": "$ref:$.channels.sketchRealtime.messages.PresenceSelection.payload",
      "GenericPresenceMessage": "$ref:$.channels.sketchRealtime.messages.PresenceSelection.payload",
      "DragBeginMessage": "$ref:$.channels.sketchRealtime.messages.DragBegin.payload",
      "DragBeginAcceptedMessage": "$ref:$.channels.sketchRealtime.messages.DragBeginAccepted.payload",
      "DragBeginRejectedMessage": "$ref:$.channels.sketchRealtime.messages.DragBeginRejected.payload",
      "DragPreviewMessage": "$ref:$.channels.sketchRealtime.messages.DragPreview.payload",
      "DragCommitMessage": "$ref:$.channels.sketchRealtime.messages.DragCommit.payload",
      "DragCancelMessage": "$ref:$.channels.sketchRealtime.messages.DragCancel.payload",
      "DragCancelledMessage": "$ref:$.channels.sketchRealtime.messages.DragCancelled.payload",
      "OpSubmitMessage": "$ref:$.channels.sketchRealtime.messages.OpSubmit.payload",
      "OpSubmitPayload": "$ref:$.channels.sketchRealtime.messages.OpSubmit.payload.properties.payload",
      "OpCommittedMessage": "$ref:$.channels.sketchRealtime.messages.OpCommitted.payload",
      "OpCommittedPayload": "$ref:$.channels.sketchRealtime.messages.OpCommitted.payload.properties.payload",
      "OpRejectedMessage": "$ref:$.channels.sketchRealtime.messages.OpRejected.payload",
      "OpRejectedPayload": "$ref:$.channels.sketchRealtime.messages.OpRejected.payload.properties.payload",
      "OpsBatchMessage": "$ref:$.channels.sketchRealtime.messages.OpsBatch.payload",
      "CommittedOperation": "$ref:$.channels.sketchRealtime.messages.OpsBatch.payload.properties.payload.properties.ops.items",
      "SketchOperation": "$ref:$.channels.sketchRealtime.messages.DragCommit.payload.properties.payload.properties.op",
      "PointRefOrNew": "$ref:$.channels.sketchRealtime.messages.DragCommit.payload.properties.payload.properties.op.properties.start",
      "Constraint": "$ref:$.channels.sketchRealtime.messages.DragCommit.payload.properties.payload.properties.op.properties.constraint",
      "Dimension": "$ref:$.channels.sketchRealtime.messages.DragCommit.payload.properties.payload.properties.op.properties.dimension",
      "SketchPatch": "$ref:$.channels.sketchRealtime.messages.OpCommitted.payload.properties.payload.properties.patch",
      "SolveStatus": "$ref:$.channels.sketchRealtime.messages.OpCommitted.payload.properties.payload.properties.solveStatus",
      "SolverDiagnostic": "$ref:$.channels.sketchRealtime.messages.OpCommitted.payload.properties.payload.properties.solveStatus.properties.diagnostics.items",
      "LockAcquireMessage": "$ref:$.channels.sketchRealtime.messages.LockAcquire.payload",
      "LockAcquirePayload": "$ref:$.channels.sketchRealtime.messages.LockAcquire.payload.properties.payload",
      "LockScope": "$ref:$.channels.sketchRealtime.messages.LockAcquire.payload.properties.payload.properties.scope",
      "LockAcquiredMessage": "$ref:$.channels.sketchRealtime.messages.LockAcquired.payload",
      "LockRejectedMessage": "$ref:$.channels.sketchRealtime.messages.LockRejected.payload",
      "LockRefreshMessage": "$ref:$.channels.sketchRealtime.messages.LockRefresh.payload",
      "LockRefreshedMessage": "$ref:$.channels.sketchRealtime.messages.LockRefreshed.payload",
      "LockReleaseMessage": "$ref:$.channels.sketchRealtime.messages.LockRelease.payload",
      "LockReleasedMessage": "$ref:$.channels.sketchRealtime.messages.LockReleased.payload",
      "StateResyncRequiredMessage": "$ref:$.channels.sketchRealtime.messages.StateResyncRequired.payload",
      "StateSnapshotMessage": "$ref:$.channels.sketchRealtime.messages.StateSnapshot.payload",
      "StatePatchMessage": "$ref:$.channels.sketchRealtime.messages.StatePatch.payload",
      "SketchDocument": "$ref:$.channels.sketchRealtime.messages.StateSnapshot.payload.properties.payload.properties.document",
      "PermissionUpdatedMessage": "$ref:$.channels.sketchRealtime.messages.PermissionUpdated.payload",
      "PermissionRevokedMessage": "$ref:$.channels.sketchRealtime.messages.PermissionRevoked.payload",
      "ConflictCreatedMessage": "$ref:$.channels.sketchRealtime.messages.ConflictCreated.payload",
      "ConflictResolvedMessage": "$ref:$.channels.sketchRealtime.messages.ConflictResolved.payload",
      "ErrorMessage": "$ref:$.channels.sketchRealtime.messages.Error.payload",
      "ErrorPayload": "$ref:$.channels.sketchRealtime.messages.Error.payload.properties.payload",
      "Vec2": "$ref:$.channels.sketchRealtime.messages.DragPreview.payload.properties.payload.properties.target"
    }
  },
  "x-parser-spec-parsed": true,
  "x-parser-api-version": 3,
  "x-parser-spec-stringified": true
};
    const config = {"show":{"sidebar":true},"sidebar":{"showOperations":"byDefault"}};
    const appRoot = document.getElementById('root');
    AsyncApiStandalone.render(
        { schema, config, }, appRoot
    );
  
