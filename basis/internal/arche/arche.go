package arche

// Queue topics — shared between producers and consumers.
const GrapheQueue = "celine:graphe:queue"

// CelineProsoponID is the fixed DB id of Celine's own prosopon record,
// seeded in 001_init.sql. Used to distinguish "assistant" messages from
// "user" messages when reconstructing history from the messages table.
const CelineProsoponID int64 = 1
