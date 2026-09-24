import { useState, useEffect } from 'react';
import { Navigate } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { adminAPI } from '../api';
import { Project, Organization, ProjectUpdate } from '../types';

type Tab = 'projects' | 'updates' | 'organizations';

const Admin = () => {
  const { user } = useAuth();
  const [activeTab, setActiveTab] = useState<Tab>('projects');
  const [pendingProjects, setPendingProjects] = useState<Project[]>([]);
  const [pendingUpdates, setPendingUpdates] = useState<ProjectUpdate[]>([]);
  const [pendingOrgs, setPendingOrgs] = useState<Organization[]>([]);
  const [loading, setLoading] = useState(true);
  // 正在处理的条目 ID：操作期间禁用按钮，避免同一名管理员重复提交
  const [processingId, setProcessingId] = useState<string | null>(null);
  // 驳回时展开原因输入框
  const [rejectingId, setRejectingId] = useState<string | null>(null);
  const [rejectReason, setRejectReason] = useState('');

  useEffect(() => {
    if (user?.role === 'admin') {
      loadPendingItems();
    }
  }, [user]);

  const loadPendingItems = async () => {
    try {
      const [projectsRes, updatesRes, orgsRes] = await Promise.all([
        adminAPI.getPendingProjects(),
        adminAPI.getPendingUpdates(),
        adminAPI.getPendingOrganizations(),
      ]);
      setPendingProjects(projectsRes.data.projects);
      setPendingUpdates(updatesRes.data.updates);
      setPendingOrgs(orgsRes.data.organizations);
    } catch (error) {
      console.error('加载待审核列表失败:', error);
    } finally {
      setLoading(false);
    }
  };

  // 提取后端错误信息；并发场景下另一位管理员已先行处理时返回 409。
  const errorMessage = (error: any, fallback: string) => {
    const msg = error?.response?.data?.message;
    if (error?.response?.status === 409 || (typeof msg === 'string' && msg.includes('already processed'))) {
      return '该动态已被其他管理员处理，列表已刷新';
    }
    return msg || fallback;
  };

  const handleReviewProject = async (projectId: string, status: 'approved' | 'rejected') => {
    setProcessingId(`project-${projectId}`);
    try {
      await adminAPI.reviewProject(projectId, { status, comment: '' });
      alert(`项目已${status === 'approved' ? '通过' : '拒绝'}`);
      await loadPendingItems();
    } catch (error) {
      alert(errorMessage(error, '操作失败'));
      await loadPendingItems();
    } finally {
      setProcessingId(null);
    }
  };

  const handleReviewUpdate = async (
    update: ProjectUpdate,
    status: 'approved' | 'rejected',
  ) => {
    const comment = rejectReason.trim();
    if (status === 'rejected' && !comment) {
      alert('驳回动态时必须填写驳回原因');
      return;
    }
    setProcessingId(`update-${update.id}`);
    try {
      await adminAPI.reviewUpdate(update.id, { status, comment });
      alert(`动态已${status === 'approved' ? '通过' : '驳回'}`);
      setRejectingId(null);
      setRejectReason('');
      await loadPendingItems();
    } catch (error) {
      // 后到的管理员：只能看到先到者的处理结果，不能重复改状态
      alert(errorMessage(error, '操作失败'));
      setRejectingId(null);
      setRejectReason('');
      await loadPendingItems();
    } finally {
      setProcessingId(null);
    }
  };

  const handleReviewOrg = async (orgId: string, status: 'approved' | 'rejected') => {
    setProcessingId(`org-${orgId}`);
    try {
      await adminAPI.reviewOrganization(orgId, { status, comment: '' });
      alert(`组织已${status === 'approved' ? '通过' : '拒绝'}`);
      await loadPendingItems();
    } catch (error) {
      alert(errorMessage(error, '操作失败'));
      await loadPendingItems();
    } finally {
      setProcessingId(null);
    }
  };

  if (!user || user.role !== 'admin') {
    return <Navigate to="/" />;
  }

  const categoryMap: Record<string, string> = {
    education: '助学',
    elderly: '助老',
    medical: '医疗',
    disaster: '救灾',
    environment: '环保',
    other: '其他',
  };

  const tabClass = (tab: Tab) =>
    `px-6 py-2 rounded-lg font-medium ${
      activeTab === tab ? 'bg-primary-600 text-white' : 'bg-white text-gray-600 border border-gray-200'
    }`;

  return (
    <div>
      <h1 className="text-3xl font-bold text-gray-900 mb-8">管理后台</h1>

      <div className="flex gap-4 mb-8 flex-wrap">
        <button onClick={() => setActiveTab('projects')} className={tabClass('projects')}>
          待审核项目 ({pendingProjects.length})
        </button>
        <button onClick={() => setActiveTab('updates')} className={tabClass('updates')}>
          待审核动态 ({pendingUpdates.length})
        </button>
        <button onClick={() => setActiveTab('organizations')} className={tabClass('organizations')}>
          待审核组织 ({pendingOrgs.length})
        </button>
      </div>

      {loading ? (
        <div className="text-center py-20">加载中...</div>
      ) : activeTab === 'projects' ? (
        <div className="bg-white rounded-2xl shadow-sm overflow-hidden">
          {pendingProjects.length === 0 ? (
            <div className="text-center py-12 text-gray-500">暂无待审核项目</div>
          ) : (
            <div className="divide-y divide-gray-100">
              {pendingProjects.map((project) => (
                <div key={project.id} className="p-6">
                  <div className="flex justify-between items-start">
                    <div className="flex-1">
                      <div className="flex items-center gap-3 mb-2">
                        <span className="px-2 py-1 bg-blue-100 text-blue-700 rounded text-xs font-medium">
                          {categoryMap[project.category]}
                        </span>
                        <span className="text-sm text-gray-500">
                          发布时间：{new Date(project.createdAt).toLocaleDateString()}
                        </span>
                      </div>
                      <h3 className="text-lg font-semibold text-gray-900 mb-2">{project.title}</h3>
                      <p className="text-gray-600 mb-2">{project.description}</p>
                      <div className="text-sm text-gray-500">
                        目标金额：¥{project.targetAmount.toLocaleString()}
                      </div>
                      <div className="text-sm text-gray-500">
                        执行计划：{project.executionPlan}
                      </div>
                    </div>
                    <div className="flex gap-2 ml-6">
                      <button
                        onClick={() => handleReviewProject(project.id, 'approved')}
                        disabled={processingId === `project-${project.id}`}
                        className="px-4 py-2 bg-green-50 text-green-600 rounded-lg hover:bg-green-100 disabled:opacity-50"
                      >
                        通过
                      </button>
                      <button
                        onClick={() => handleReviewProject(project.id, 'rejected')}
                        disabled={processingId === `project-${project.id}`}
                        className="px-4 py-2 bg-red-50 text-red-600 rounded-lg hover:bg-red-100 disabled:opacity-50"
                      >
                        拒绝
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      ) : activeTab === 'updates' ? (
        <div className="bg-white rounded-2xl shadow-sm overflow-hidden">
          {pendingUpdates.length === 0 ? (
            <div className="text-center py-12 text-gray-500">暂无待审核动态</div>
          ) : (
            <div className="divide-y divide-gray-100">
              {pendingUpdates.map((update) => (
                <div key={update.id} className="p-6">
                  <div className="flex justify-between items-start gap-6">
                    <div className="flex-1">
                      <div className="flex items-center gap-3 mb-2 flex-wrap">
                        <span className="px-2 py-1 bg-amber-100 text-amber-700 rounded text-xs font-medium">
                          待审核
                        </span>
                        <span className="text-sm font-medium text-gray-700">
                          {update.project?.organization?.name || '所属组织'}
                        </span>
                        <span className="text-sm text-gray-400">/</span>
                        <span className="text-sm text-gray-500">
                          项目：{update.project?.title || update.projectId}
                        </span>
                        <span className="text-sm text-gray-500">
                          提交时间：{new Date(update.createdAt).toLocaleString()}
                        </span>
                      </div>
                      <h3 className="text-lg font-semibold text-gray-900 mb-2">{update.title}</h3>
                      <p className="text-gray-600 whitespace-pre-wrap">{update.content}</p>

                      {rejectingId === update.id && (
                        <div className="mt-4">
                          <label className="block text-sm font-medium text-gray-700 mb-2">
                            驳回原因 *（提交后组织可在自己的项目中看到）
                          </label>
                          <textarea
                            value={rejectReason}
                            onChange={(e) => setRejectReason(e.target.value)}
                            rows={3}
                            autoFocus
                            className="w-full px-4 py-3 border border-gray-200 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent resize-none"
                            placeholder="请写明驳回原因，便于组织修改后重新提交"
                          />
                        </div>
                      )}
                    </div>
                    <div className="flex gap-2 shrink-0">
                      {rejectingId === update.id ? (
                        <>
                          <button
                            onClick={() => handleReviewUpdate(update, 'rejected')}
                            disabled={processingId === `update-${update.id}`}
                            className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 disabled:opacity-50"
                          >
                            确认驳回
                          </button>
                          <button
                            onClick={() => {
                              setRejectingId(null);
                              setRejectReason('');
                            }}
                            disabled={processingId === `update-${update.id}`}
                            className="px-4 py-2 border border-gray-200 text-gray-600 rounded-lg hover:bg-gray-50 disabled:opacity-50"
                          >
                            取消
                          </button>
                        </>
                      ) : (
                        <>
                          <button
                            onClick={() => handleReviewUpdate(update, 'approved')}
                            disabled={processingId === `update-${update.id}`}
                            className="px-4 py-2 bg-green-50 text-green-600 rounded-lg hover:bg-green-100 disabled:opacity-50"
                          >
                            通过
                          </button>
                          <button
                            onClick={() => {
                              setRejectingId(update.id);
                              setRejectReason('');
                            }}
                            disabled={processingId === `update-${update.id}`}
                            className="px-4 py-2 bg-red-50 text-red-600 rounded-lg hover:bg-red-100 disabled:opacity-50"
                          >
                            驳回
                          </button>
                        </>
                      )}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      ) : (
        <div className="bg-white rounded-2xl shadow-sm overflow-hidden">
          {pendingOrgs.length === 0 ? (
            <div className="text-center py-12 text-gray-500">暂无待审核组织</div>
          ) : (
            <div className="divide-y divide-gray-100">
              {pendingOrgs.map((org) => (
                <div key={org.id} className="p-6">
                  <div className="flex justify-between items-start">
                    <div className="flex-1">
                      <h3 className="text-lg font-semibold text-gray-900 mb-2">{org.name}</h3>
                      <p className="text-gray-600 mb-2">{org.description}</p>
                      <div className="grid grid-cols-2 gap-4 text-sm text-gray-500">
                        <div>执照编号：{org.licenseNumber || '-'}</div>
                        <div>联系人：{org.contactPerson || '-'}</div>
                        <div>联系电话：{org.contactPhone || '-'}</div>
                        <div>地址：{org.address || '-'}</div>
                      </div>
                    </div>
                    <div className="flex gap-2 ml-6">
                      <button
                        onClick={() => handleReviewOrg(org.id, 'approved')}
                        disabled={processingId === `org-${org.id}`}
                        className="px-4 py-2 bg-green-50 text-green-600 rounded-lg hover:bg-green-100 disabled:opacity-50"
                      >
                        通过
                      </button>
                      <button
                        onClick={() => handleReviewOrg(org.id, 'rejected')}
                        disabled={processingId === `org-${org.id}`}
                        className="px-4 py-2 bg-red-50 text-red-600 rounded-lg hover:bg-red-100 disabled:opacity-50"
                      >
                        拒绝
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
};

export default Admin;
