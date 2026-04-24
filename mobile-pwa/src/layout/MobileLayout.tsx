import { Outlet } from 'react-router-dom';
import { TopBar } from './TopBar';
import { TabBar } from './TabBar';

export function MobileLayout() {
  return (
    <div className="m-screen">
      <TopBar />
      <main className="m-screen-body">
        <Outlet />
      </main>
      <TabBar />
    </div>
  );
}
