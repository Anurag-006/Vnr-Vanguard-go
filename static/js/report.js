document.addEventListener("DOMContentLoaded", async () => {
    // 1. Grab the roll number from the URL
    const urlParams = new URLSearchParams(window.location.search);
    const roll = urlParams.get('roll');
    const exam = urlParams.get('exam');

    if (!roll) {
        document.getElementById('loader').innerText = "Error: No Roll Number Provided.";
        return;
    }

    if (!exam) {
        document.getElementById('loader').innerText = "Error: No Exam Provided.";
        return;
    }

    try {
        // 2. Fetch the JSON from our Go API
        const response = await fetch(`/api/v1/report?roll=${roll}&exam=${exam}`);
        
        if (!response.ok) throw new Error("Result Withheld or Roll Number Not Found");
        
        const data = await response.json();

        // 3. Swap UI states (Hide loader, show content)
        document.getElementById('loader').style.display = 'none';
        document.getElementById('report-content').style.display = 'block';

        // 4. Update the Profile Card
        document.title = `${data.name} | Transcript`;
        document.getElementById('ui-name').innerText = data.name;
        document.getElementById('ui-roll').innerText = data.roll;
        document.getElementById('ui-sgpa').innerText = data.sgpa;

        // Update Verdict Text
        const verdictEl = document.getElementById('ui-verdict');
        verdictEl.innerText = `Final Verdict: ${data.verdict}`;
        verdictEl.style.color = data.verdict.includes("PASS") ? "var(--success)" : "var(--danger)";

        // 5. Draw the Subjects Table
        const tbody = document.getElementById('ui-table-body');
        tbody.innerHTML = ''; // Clear anything inside
        
        data.subjects.forEach(sub => {
            // Determine class based on Result text
            const resultClass = sub.result.includes('PASS') ? 'result-pass' : 'result-fail';
            
            const tr = document.createElement('tr');
            tr.innerHTML = `
                <td>
                    <div style="font-weight: 600;">${sub.title}</div>
                    <div style="font-size: 0.8rem; color: var(--text-dim);">Code: ${sub.code}</div>
                </td>
                <td><span class="grade-badge">${sub.grade}</span></td>
                <td><span class="point-badge">${sub.points}</span></td> 
                <td class="${resultClass}">${sub.result}</td>
            `;
            tbody.appendChild(tr);
        });

    } catch (error) {
        document.getElementById('loader').innerText = `Vanguard Error: ${error.message}`;
        document.getElementById('loader').style.color = "var(--danger)";
    }
});