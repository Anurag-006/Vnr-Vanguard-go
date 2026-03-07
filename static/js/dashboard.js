document.addEventListener("DOMContentLoaded", () => {
    // ==========================================
    // 🧠 STATE MANAGEMENT
    // ==========================================
    let currentBatchData = [];
    let currentExamId = "";

    // ==========================================
    // 🎯 DOM ELEMENTS
    // ==========================================
    const searchForm = document.getElementById('searchForm');
    const searchInput = document.getElementById('searchInput');
    const statusFilter = document.getElementById('statusFilter');
    const subjectFilter = document.getElementById('subjectFilter');
    const sectionInput = document.getElementById('sectionInput');
    const yearInput = document.getElementById('yearInput');
    const examInput = document.getElementById('examInput');
    const exportCsvBtn = document.getElementById('exportCsvBtn'); 
    
    const tableBody = document.getElementById('tableBody');
    const emptyState = document.getElementById('emptyState');
    const tableWrapper = document.getElementById('tableWrapper');
    const filterCard = document.getElementById('filterCard'); 
    const searchRow = document.getElementById('searchRow');
    const metaRow = document.getElementById('metaRow');
    const metaBadge = document.getElementById('metaBadge');
    const studentCount = document.getElementById('studentCount');
    const loadingOverlay = document.getElementById('loadingOverlay'); 
    const statsLink = document.getElementById('statsLink');

    // ==========================================
    // 🎧 EVENT LISTENERS
    // ==========================================
    searchForm.addEventListener('submit', handleScrapeRequest);
    searchInput.addEventListener('input', renderTable);
    statusFilter.addEventListener('change', renderTable);
    subjectFilter.addEventListener('change', renderTable);
    exportCsvBtn.addEventListener('click', exportToCSV);

    // ==========================================
    // 🚀 INITIALIZATION
    // ==========================================
    async function initApp() {
        try {
            const [secRes, examRes] = await Promise.all([
                fetch('/api/v1/sections'),
                fetch('/api/v1/exams')
            ]);
            const secData = await secRes.json();
            const examData = await examRes.json();
            
            sectionInput.innerHTML = '<option value="" disabled selected>Select Dept</option>';
            secData.sections.forEach(sec => {
                sectionInput.innerHTML += `<option value="${sec}">${sec}</option>`;
            });

            examInput.innerHTML = '<option value="" disabled selected>Select Exam</option>';
            examData.exams.forEach(exam => {
                examInput.innerHTML += `<option value="${exam.id}">${exam.name} (${exam.id})</option>`;
            });
        } catch (e) { 
            console.error("Vanguard Init Error", e); 
        }
    }
    initApp();

    // ==========================================
    // ⚡ CORE FETCH LOGIC
    // ==========================================
    async function handleScrapeRequest(e) {
        e.preventDefault();
        const section = sectionInput.value;
        const year = yearInput.value;
        currentExamId = examInput.value;

        if(loadingOverlay) loadingOverlay.style.display = 'flex';
        emptyState.style.display = 'none';

        try {
            const res = await fetch(`/api/v1/class?section=${section}&year=${year}&exam=${currentExamId}`);
            const data = await res.json();

            if (!res.ok) {
                if(res.status === 429) throw new Error("Rate Limit: Please wait 60 seconds.");
                throw new Error(data.error || "Failed to fetch results.");
            }

            currentBatchData = data.leaderboard;
            
            // --- 🚀 THE FIX: REVEAL HIDDEN UI ELEMENTS ---
            if(loadingOverlay) loadingOverlay.style.display = 'none';
            tableWrapper.style.display = 'block';
            if(filterCard) filterCard.style.display = 'block';
            if(searchRow) searchRow.style.display = 'block';
            if(metaRow) metaRow.style.display = 'flex';
            if(exportCsvBtn) exportCsvBtn.style.display = 'block'; // This makes the button visible
            
            if(studentCount) studentCount.innerText = `Showing ${data.meta.students_found} students`;
            if(statsLink) statsLink.href = `/stats?section=${section}&year=${year}&exam=${currentExamId}`;

            // Handle Meta Badge
const source = data.meta.source;

metaBadge.className = "update-badge";

if (source === "memory_cache") {
    metaBadge.innerHTML = "⚡ L1 RAM HIT";
    metaBadge.classList.add("badge-l1");

} else if (source === "redis_cache") {
    metaBadge.innerHTML = "🗄️ L2 REDIS CACHE";
    metaBadge.classList.add("badge-l2");

} else {
    metaBadge.innerHTML = "🌐 LIVE ORIGIN FETCH";
    metaBadge.classList.add("badge-live");
}

/* trigger slide animation */
requestAnimationFrame(() => {
    metaBadge.classList.add("badge-show");
});

            populateSubjectDropdown();
            renderTable();

        } catch (error) {
            if(loadingOverlay) loadingOverlay.style.display = 'none';
            emptyState.style.display = 'block';
            emptyState.innerHTML = `<span>⚠️</span><h2>Vanguard Error</h2><p style="color:var(--danger)">${error.message}</p>`;
        }
    }

    // ==========================================
    // 📊 CSV EXPORTER (Full Matrix Mode)
    // ==========================================
function exportToCSV() {
    if (!currentBatchData || currentBatchData.length === 0) return;

    let allSubjectTitles = new Set();
    currentBatchData.forEach(s => s.subjects?.forEach(sub => allSubjectTitles.add(sub.title)));
    let sortedSubjects = Array.from(allSubjectTitles).sort();

    const headers = ['Rank', 'Roll Number', 'Name', ...sortedSubjects, 'SGPA', 'Verdict', 'Backlogs'];
    let csvContent = headers.map(h => `"${h}"`).join(",") + "\n";

    currentBatchData.forEach((student, index) => {
        let pointMap = {}; 
        
        if (student.subjects) {
            student.subjects.forEach(sub => {
                // 🚀 FORCE CHECK: Explicitly use .points property
                // We use parseFloat to ensure it's a valid number string
                let pts = sub.points ? sub.points.trim() : "0";
                pointMap[sub.title] = pts;
            });
        }
        
        let backlogCount = student.subjects ? student.subjects.filter(sub => sub.result.includes("FAIL")).length : 0;
        const isWithheld = student.name === "Result Withheld" || student.sgpa === "0.00" || student.sgpa === "";
        let verdict = backlogCount > 0 ? "FAIL" : "PASS";
        if (isWithheld) verdict = "WITHHELD";

        const row = [
            index + 1,
            student.roll,
            `"${student.name}"`,
            // 🚀 MAP POINTS: Access the pointMap
            ...sortedSubjects.map(sub => pointMap[sub] || "0"),
            isWithheld ? "W.H." : student.sgpa,
            verdict,
            backlogCount
        ];
        
        csvContent += row.join(",") + "\n";
    });

    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `Vanguard_Points_Matrix_${sectionInput.value}_${yearInput.value}.csv`;
    link.click();
}

    // ==========================================
    // 🎨 UI RENDERING
    // ==========================================
    function populateSubjectDropdown() {
        subjectFilter.innerHTML = '<option value="all">Sort by Subject Toppers...</option>';
        let subs = new Set();
        currentBatchData.forEach(s => s.subjects?.forEach(sub => subs.add(sub.title)));
        Array.from(subs).sort().forEach(sub => {
            subjectFilter.innerHTML += `<option value="${sub}">${sub}</option>`;
        });
    }

    function renderTable() {
        const term = searchInput.value.toUpperCase();
        const status = statusFilter.value;
        const targetSub = subjectFilter.value;

        let filtered = currentBatchData.filter(s => {
            const matchesSearch = s.name.toUpperCase().includes(term) || s.roll.includes(term);
            const hasFail = s.subjects && s.subjects.some(sub => sub.result.includes("FAIL"));
            const matchesStatus = status === 'all' || (status === 'pass' && !hasFail) || (status === 'fail' && hasFail);
            const matchesSubject = targetSub === 'all' || (s.subjects && s.subjects.some(sub => sub.title === targetSub));
            return matchesSearch && matchesStatus && matchesSubject;
        });

        if (targetSub !== 'all') {
            filtered.sort((a, b) => {
                const gpA = a.subjects.find(sub => sub.title === targetSub)?.points || 0;
                const gpB = b.subjects.find(sub => sub.title === targetSub)?.points || 0;
                return parseFloat(gpB) - parseFloat(gpA);
            });
        }

        tableBody.innerHTML = filtered.map((s, index) => {
            const isWithheld = s.name === "Result Withheld" || s.sgpa === "0.00" || s.sgpa === "";
            
            let scoreDisplay = isWithheld ? "W.H." : s.sgpa;
            let isPerfect = !isWithheld && parseFloat(s.sgpa) >= 10.0;

            if (targetSub !== 'all' && s.subjects) {
                const subObj = s.subjects.find(sub => sub.title === targetSub);
                if (subObj) {
                    scoreDisplay = `${subObj.grade} (${subObj.points})`;
                    isPerfect = parseFloat(subObj.points) >= 10;
                } else {
                    scoreDisplay = "N/A";
                    isPerfect = false;
                }
            }

            const rowStyle = isWithheld ? 'opacity: 0.6; background: rgba(239, 68, 68, 0.05);' : '';
            const pillClass = isWithheld ? 'sgpa-pill withheld' : 'sgpa-pill';

            return `
                <tr style="${rowStyle}">
                    <td><span class="rank-badge">#${index + 1}</span></td>
                    <td style="color:var(--text-dim); font-family:monospace;">${s.roll}</td>
                    <td>
                        <a href="/report?roll=${s.roll}&exam=${currentExamId}" class="student-name-link">${s.name}</a>
                    </td>
                    <td>
                        <span class="${pillClass}">${scoreDisplay}</span>
                        ${isPerfect ? ' <span class="trophy-icon">🏆</span>' : ''}
                    </td>
                </tr>
            `;
        }).join('');
    }
});