// Closed candles a history load keeps per coin and interval; synchronization
// already keeps 2,000.
export const historyLoadDepth = { default: 5000, max: 20000, min: 2001 };
// How often a running history load is polled, in milliseconds.
export const historyLoadPollInterval = 2000;
// The most coins one history load accepts.
export const maxHistoryLoadCoins = 10;
