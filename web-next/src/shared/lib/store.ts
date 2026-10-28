import { configureStore } from '@reduxjs/toolkit';
import { setupListeners } from '@reduxjs/toolkit/query';
import { api } from '@/shared/api';
/** makeStore creates the Redux store with the API slice. */
export function makeStore() {
  return configureStore({
    reducer: { [api.reducerPath]: api.reducer },
    middleware: (getDefault) => getDefault().concat(api.middleware),
    devTools: false,
  });
}
// setupListeners turns on refetching on focus and reconnect.
export { setupListeners };
// Types of the store, for typed hooks.
export type AppStore = ReturnType<typeof makeStore>;
export type RootState = ReturnType<AppStore['getState']>;
export type AppDispatch = AppStore['dispatch'];
